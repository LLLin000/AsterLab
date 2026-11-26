package main

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"asterlab-mygo-prototype/internal/astercore"
	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/crashguard"
	"asterlab-mygo-prototype/internal/recovery"
	"asterlab-mygo-prototype/internal/shell"
	"asterlab-mygo-prototype/internal/syncengine"
)

func makeM123TestPNGBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8((x * 7) % 255),
				G: uint8((y * 11) % 255),
				B: uint8(((x + y) * 5) % 255),
				A: 255,
			})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	return buf.Bytes()
}

func makeM123TestPlateXLSX(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	files := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>` +
			`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
			`<Default Extension="xml" ContentType="application/xml"/>` +
			`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>` +
			`<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>` +
			`<Override PartName="/xl/sharedStrings.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml"/>` +
			`</Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>` +
			`</Relationships>`,
		"xl/workbook.xml": `<?xml version="1.0" encoding="UTF-8"?>` +
			`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
			`<sheets><sheet name="Plate1" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>` +
			`</Relationships>`,
		"xl/sharedStrings.xml": `<?xml version="1.0" encoding="UTF-8"?>` +
			`<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="2" uniqueCount="2">` +
			`<si><t>A</t></si><si><t>B</t></si></sst>`,
		"xl/worksheets/sheet1.xml": `<?xml version="1.0" encoding="UTF-8"?>` +
			`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>` +
			`<row r="1"><c r="B1"><v>1</v></c><c r="C1"><v>2</v></c><c r="D1"><v>3</v></c></row>` +
			`<row r="2"><c r="A2" t="s"><v>0</v></c><c r="B2"><v>0.91</v></c><c r="C2"><v>0.95</v></c><c r="D2"><v>0.99</v></c></row>` +
			`<row r="3"><c r="A3" t="s"><v>1</v></c><c r="B3"><v>1.41</v></c><c r="C3"><v>1.44</v></c><c r="D3"><v>1.48</v></c></row>` +
			`</sheetData></worksheet>`,
	}
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func newM123TestApp(t *testing.T, src string) (*app, shell.WorkspaceTab) {
	t.Helper()
	vRoot := t.TempDir()
	sh := &shell.Shell{
		TodayDate:          "2026-11-26",
		SelectedDate:       "2026-11-26",
		DefaultPlateFormat: 96,
		DefaultExcessPct:   20,
	}
	a := &app{shell: sh}
	a.installCrashGuard()
	if err := a.initVaultWorkspace(vRoot); err != nil {
		t.Fatalf("initVaultWorkspace: %v", err)
	}
	a.wireRecoveryHost()
	entry, err := a.vault.CreateExperiment("Proj", "M123 Assay", "2026-11-26", src)
	if err != nil {
		t.Fatalf("CreateExperiment: %v", err)
	}
	a.refreshVaultTreeAndCalendar()
	sh.EnsureTabs()
	sh.OpenFileInPane(sh.ActivePaneID, entry.RelPath, entry.Title, shell.ModeEdit, false)
	tab := sh.ActiveTab()
	if tab == nil {
		t.Fatal("expected active tab")
	}
	return a, *tab
}

func TestM123MobileToolbarActionDispatchAndCameraPhoto(t *testing.T) {
	src := "---\ntitle: M123 Mobile Toolbar\nscheduled_date: 2026-11-26\nstage: in_progress\n---\n\n#@ 1. 细胞处理\n37°C 避光孵育 30 min。\n第二行观察记录\n\n```asterlab\nkind: param_table\nid: pt-1\nschema: 1\ntitle: 培养参数\nrows:\n  - key: temp\n    label: 温度\n    value: 37\n    unit: Cel\n```\n\n```asterlab\nkind: timer\nid: tm-1\nschema: 1\ntitle: 孵育计时\nmode: countdown\nduration:\n  value: 15\n  unit: min\n```\n"
	a, tab := newM123TestApp(t, src)
	ld, err := a.docStore.Open(tab.FilePath)
	if err != nil {
		t.Fatal(err)
	}

	var v *asterdoc.View
	v = &asterdoc.View{
		Doc:           ld.Parsed,
		Registry:      asterdoc.BuiltinRegistry(),
		Mode:          asterdoc.ModeDesign,
		UnifiedLive:   true,
		MobileCompact: true,
		Allowed:       map[string]bool{asterdoc.PermDocumentEdit: true},
		PantryCompletionItems: []asterdoc.PantryMentionItem{
			{ID: "p-1", Name: "VEGF-165", LabelCode: "R-1", Location: "-80°C", ItemType: "reagent", Status: "normal"},
		},
		OnChange: func(next string) {
			_ = ld.ApplyEdit(next)
			v.Doc = ld.Parsed
		},
	}
	segStart := strings.Index(ld.Source, "37°C 避光孵育 30 min。")
	segText := "37°C 避光孵育 30 min。\n第二行观察记录"
	st := v.SetTestLiveEditor("prose-m123", segText, segStart, len("37°C "), func(nextText string) string {
		nextSrc, rErr := asterdoc.ReplaceByteRange(ld.Source, segStart, segStart+len(segText), nextText)
		if rErr != nil {
			t.Fatalf("ReplaceByteRange: %v", rErr)
		}
		v.OnChange(nextSrc)
		segText = nextText
		return nextSrc
	})

	a.ensureLiveViewsMap()
	a.liveViews[a.shell.ActivePaneID+"|"+tab.ID+"|"+tab.FilePath] = v
	a.shell.OnMobileDocToolbarAction = func(curTab shell.WorkspaceTab, action string) {
		a.handleMobileDocToolbarAction(nil, curTab, action)
	}

	// Verify cursor, formatting, indent, footnote, mention, slash, NL, block move, and fold actions
	pos0 := st.Cursor()
	a.shell.OnMobileDocToolbarAction(tab, "cursor_left")
	if st.Cursor() >= pos0 {
		t.Fatalf("expected cursor_left to move left from %d, got %d", pos0, st.Cursor())
	}
	a.shell.OnMobileDocToolbarAction(tab, "cursor_right")
	if st.Cursor() != pos0 {
		t.Fatalf("expected cursor_right back to %d, got %d", pos0, st.Cursor())
	}

	a.shell.OnMobileDocToolbarAction(tab, "heading")
	if !strings.HasPrefix(st.Buffer.String(), "# ") {
		t.Fatalf("heading action did not apply H1: %q", st.Buffer.String())
	}
	a.shell.OnMobileDocToolbarAction(tab, "heading")
	a.shell.OnMobileDocToolbarAction(tab, "heading")
	a.shell.OnMobileDocToolbarAction(tab, "heading") // cycle back to plain

	st.Select(0, len("37°C"))
	a.shell.OnMobileDocToolbarAction(tab, "bold")
	if !strings.Contains(st.Buffer.String(), "**37°C**") {
		t.Fatalf("bold action failed: %q", st.Buffer.String())
	}
	a.shell.OnMobileDocToolbarAction(tab, "bold") // unwrap bold so NL phrase stays clean
	st.Select(len("37°C "), len("37°C "))

	a.shell.OnMobileDocToolbarAction(tab, "nl_structure")
	if v.ActiveMobileSheetKind() != "nl" {
		t.Fatalf("nl_structure expected nl sheet, got %q", v.ActiveMobileSheetKind())
	}
	v.DismissActiveMobileSheet()

	a.shell.OnMobileDocToolbarAction(tab, "bullet_list")
	if !strings.HasPrefix(st.Buffer.String(), "- ") {
		t.Fatalf("bullet_list failed: %q", st.Buffer.String())
	}
	a.shell.OnMobileDocToolbarAction(tab, "checklist")
	if !strings.HasPrefix(st.Buffer.String(), "- [ ] ") {
		t.Fatalf("checklist failed: %q", st.Buffer.String())
	}
	a.shell.OnMobileDocToolbarAction(tab, "indent")
	if !strings.HasPrefix(st.Buffer.String(), "  - [ ] ") {
		t.Fatalf("indent failed: %q", st.Buffer.String())
	}
	a.shell.OnMobileDocToolbarAction(tab, "outdent")
	if !strings.HasPrefix(st.Buffer.String(), "- [ ] ") {
		t.Fatalf("outdent failed: %q", st.Buffer.String())
	}

	a.shell.OnMobileDocToolbarAction(tab, "move_block_down")
	if !strings.HasPrefix(st.Buffer.String(), "第二行观察记录") {
		t.Fatalf("move_block_down failed: %q", st.Buffer.String())
	}
	a.shell.OnMobileDocToolbarAction(tab, "move_block_up")
	if !strings.HasPrefix(st.Buffer.String(), "- [ ] ") {
		t.Fatalf("move_block_up failed: %q", st.Buffer.String())
	}

	a.shell.OnMobileDocToolbarAction(tab, "fold_block")
	a.shell.OnMobileDocToolbarAction(tab, "fold_block")

	a.shell.OnMobileDocToolbarAction(tab, "at_entity")
	if v.ActiveMobileSheetKind() != "mention" {
		t.Fatalf("at_entity expected mention sheet, got %q", v.ActiveMobileSheetKind())
	}
	v.DismissActiveMobileSheet()
	a.shell.OnMobileDocToolbarAction(tab, "slash_module")
	if v.ActiveMobileSheetKind() != "slash" {
		t.Fatalf("slash_module expected slash sheet, got %q", v.ActiveMobileSheetKind())
	}
	v.DismissActiveMobileSheet()
	a.shell.OnMobileDocToolbarAction(tab, "footnote")
	if v.ActiveMobileSheetKind() != "footnote" || !strings.Contains(ld.Source, "[^1]") {
		t.Fatalf("footnote action failed: sheet=%q src=%s", v.ActiveMobileSheetKind(), ld.Source)
	}
	v.DismissActiveMobileSheet()

	// Verify camera_photo: imports lossless original + generates syncengine preview sidecar + inserts figure@1
	photoPath := filepath.Join(t.TempDir(), "huvec_capture.png")
	if err := os.WriteFile(photoPath, makeM123TestPNGBytes(t, 1600, 1000), 0o644); err != nil {
		t.Fatal(err)
	}
	a.capturePhotoFn = func(ctx context.Context) (string, error) {
		return photoPath, nil
	}
	a.shell.OnMobileDocToolbarAction(tab, "camera_photo")

	reloaded, err := a.docStore.Open(tab.FilePath)
	if err != nil {
		t.Fatal(err)
	}
	var figSrc string
	for _, m := range reloaded.Parsed.Modules {
		if m.Kind == "figure" {
			figSrc, _ = m.Data["src"].(string)
		}
	}
	if figSrc == "" || !strings.HasPrefix(figSrc, "./assets/") {
		t.Fatalf("expected camera_photo to insert figure@1 with ./assets/ src, got %q", figSrc)
	}
	origBytes, err := os.ReadFile(a.vault.AbsPath(strings.TrimPrefix(figSrc, "./")))
	if err != nil || len(origBytes) == 0 {
		t.Fatalf("expected lossless original in vault assets, err=%v", err)
	}
	prevRel := syncengine.CanonicalPreviewPath(syncengine.SHA256Bytes(origBytes))
	if prevInfo, err := os.Stat(a.vault.AbsPath(prevRel)); err != nil || prevInfo.Size() == 0 {
		t.Fatalf("expected preview sidecar at %s, err=%v", prevRel, err)
	}
}

func TestM123LabIOImportExportAndShareWiring(t *testing.T) {
	src := "---\ntitle: M123 LabIO 导出测试\nscheduled_date: 2026-11-26\nstage: in_progress\n---\n\n#@ 1. 分组与孔板\n本实验记录细胞活力测定[^1]。\n\n```asterlab\nkind: group_table\nid: gt-1\nschema: 1\ntitle: 分组\nrows:\n  - id: g_ctrl\n    name: 对照组\n    role: control\n  - id: g_trt\n    name: 给药组\n    role: treatment\n```\n\n```asterlab\nkind: plate\nid: plate-1\nschema: 1\ntitle: 96孔板\nformat: 96\ngroups:\n  g_ctrl: [A1, A2, A3]\n  g_trt: [B1, B2, B3]\n```\n\n[^1]: 孔板读数采用 OD450 测定。\n"
	a, tab := newM123TestApp(t, src)
	a.shell.OnMobileDocToolbarAction = func(curTab shell.WorkspaceTab, action string) {
		a.handleMobileDocToolbarAction(nil, curTab, action)
	}

	// Embed a tiny PNG figure so Docx export also exercises embedded PNG packaging.
	tinyRel, err := a.vault.ImportAssetBytes(makeM123TestPNGBytes(t, 64, 48), "well_fig.png")
	if err != nil {
		t.Fatal(err)
	}
	ld, _ := a.docStore.Open(tab.FilePath)
	withFig := ld.Source + "\n```asterlab\nkind: figure\nid: fig-1\nschema: 1\ntitle: 显微图\nsrc: " + tinyRel + "\ncaption: 细胞形态\n```\n"
	if err := ld.ApplyEdit(withFig); err != nil {
		t.Fatal(err)
	}

	// 1. Import CSV plate matrix via import_plate_data
	csvFile := filepath.Join(t.TempDir(), "plate_csv.csv")
	if err := os.WriteFile(csvFile, []byte(",1,2,3\nA,0.51,0.52,0.53\nB,1.21,1.25,1.29\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.openImportFileFn = func(title string) (string, error) { return csvFile, nil }
	a.shell.OnMobileDocToolbarAction(tab, "import_plate_data")

	// 2. Import XLSX plate matrix via import_plate_data
	xlsxFile := filepath.Join(t.TempDir(), "plate_xlsx.xlsx")
	if err := os.WriteFile(xlsxFile, makeM123TestPlateXLSX(t), 0o644); err != nil {
		t.Fatal(err)
	}
	a.openImportFileFn = func(title string) (string, error) { return xlsxFile, nil }
	a.shell.OnMobileDocToolbarAction(tab, "import_plate_data")

	dtCount := 0
	for _, m := range ld.Parsed.Modules {
		if m.Kind == "data_table" {
			dtCount++
		}
	}
	if dtCount != 2 {
		t.Fatalf("expected 2 imported data_table modules (CSV + XLSX), got %d", dtCount)
	}

	// 3. Export Tidy CSV
	savedExports := map[string][]byte{}
	a.saveExportFileFn = func(defaultName, title string, data []byte) (string, error) {
		p := filepath.Join(t.TempDir(), defaultName)
		if wErr := os.WriteFile(p, data, 0o644); wErr != nil {
			return "", wErr
		}
		savedExports[defaultName] = append([]byte(nil), data...)
		return p, nil
	}
	a.shell.OnMobileDocToolbarAction(tab, "export_tidy_csv")
	var csvOut []byte
	for k, v := range savedExports {
		if strings.HasSuffix(k, "-tidy.csv") {
			csvOut = v
		}
	}
	if !bytes.HasPrefix(csvOut, []byte{0xef, 0xbb, 0xbf}) || !strings.Contains(string(csvOut), "对照组") || !strings.Contains(string(csvOut), "A1") {
		t.Fatalf("unexpected Tidy CSV output: %q", string(csvOut))
	}

	// 4. Export Word (.docx)
	a.shell.OnMobileDocToolbarAction(tab, "export_docx")
	var docxOut []byte
	for k, v := range savedExports {
		if strings.HasSuffix(k, ".docx") {
			docxOut = v
		}
	}
	zr, err := zip.NewReader(bytes.NewReader(docxOut), int64(len(docxOut)))
	if err != nil {
		t.Fatalf("exported docx is not a valid zip: %v", err)
	}
	partNames := map[string]bool{}
	for _, f := range zr.File {
		partNames[f.Name] = true
	}
	for _, wantPart := range []string{"[Content_Types].xml", "word/document.xml", "word/styles.xml", "word/footnotes.xml", "word/media/image1.png"} {
		if !partNames[wantPart] {
			t.Fatalf("docx missing part %s; got %v", wantPart, partNames)
		}
	}

	// 5. Share doc via ShareFile hook + ErrUnsupported fallback
	sharedPath := ""
	a.shareFileFn = func(path, text string) error {
		sharedPath = path
		return nil
	}
	a.shell.OnMobileDocToolbarAction(tab, "share_doc")
	if sharedPath == "" {
		t.Fatal("expected share_doc to invoke ShareFile with an exported path")
	}
	if _, err := os.Stat(sharedPath); err != nil {
		t.Fatalf("shared file missing at %s: %v", sharedPath, err)
	}
}

func TestM123RecoveryHostAndCrashGuard(t *testing.T) {
	srcV1 := "---\ntitle: Recovery Doc v1\nscheduled_date: 2026-11-26\n---\n\n# 初始方案 v1\n"
	a, tab := newM123TestApp(t, srcV1)

	// 1. Snapshot + restore via OnRestoreSnapshot
	snap1, created, err := a.snapshotDocFile(tab.FilePath, "manual", true)
	if err != nil || !created {
		t.Fatalf("snapshotDocFile v1 failed: created=%v err=%v", created, err)
	}
	ld, _ := a.docStore.Open(tab.FilePath)
	srcV2 := "---\ntitle: Recovery Doc v2\nscheduled_date: 2026-11-26\n---\n\n# 修改方案 v2\n"
	if err := ld.ApplyEdit(srcV2); err != nil {
		t.Fatal(err)
	}
	a.shell.OnRestoreSnapshot(tab.FilePath, snap1.ID)
	diskAfterRestore, _ := os.ReadFile(a.vault.AbsPath(tab.FilePath))
	if !strings.Contains(string(diskAfterRestore), "初始方案 v1") {
		t.Fatalf("expected OnRestoreSnapshot to restore v1, got:\n%s", string(diskAfterRestore))
	}

	// 2. CrashGuard sanitized panic log + emergency draft recovery
	a.crashGuardLogDir = filepath.Join(t.TempDir(), "crashlogs")
	a.installCrashGuard()
	logPath := a.recordCrashGuardPanic("test", "boom with Bearer secret_token_123456 and https://user:mySecretPass@example.com/dav")
	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("expected crash.log written at %s: %v", logPath, err)
	}
	if strings.Contains(string(logBytes), "secret_token_123456") || strings.Contains(string(logBytes), "mySecretPass") {
		t.Fatalf("crash.log leaked secrets: %s", string(logBytes))
	}

	unsavedDraft := "---\ntitle: Recovery Draft\nscheduled_date: 2026-11-26\n---\n\n# 崩溃前未保存内容\n"
	a.stageEmergencyDraft(tab.FilePath, unsavedDraft, 12)
	recovered, err := a.checkAndRecoverEmergencyDrafts()
	if err != nil || len(recovered) != 1 {
		t.Fatalf("checkAndRecoverEmergencyDrafts = (%d, %v), want 1", len(recovered), err)
	}
	diskRecovered, _ := os.ReadFile(a.vault.AbsPath(tab.FilePath))
	if string(diskRecovered) != unsavedDraft {
		t.Fatalf("expected recovered draft on disk, got:\n%s", string(diskRecovered))
	}
	if rem, _ := crashguard.ListEmergencyDrafts(a.vault.Root); len(rem) != 0 {
		t.Fatalf("expected staged emergency drafts cleared after recovery, got %d", len(rem))
	}

	// 3. Trash collision-safe restore via OnVaultRestoreTrash
	trItem, err := a.vault.Trash(tab.FilePath)
	if err != nil {
		t.Fatal(err)
	}
	// Create a new file at same path before restoring trash -> must not clobber!
	if err := os.WriteFile(a.vault.AbsPath(tab.FilePath), []byte("newer file"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.shell.OnVaultRestoreTrash(trItem.TrashName)
	if cur, _ := os.ReadFile(a.vault.AbsPath(tab.FilePath)); string(cur) != "newer file" {
		t.Fatalf("expected newer file preserved on collision, got %q", string(cur))
	}

	// 4. Full Vault Zip Backup & Atomic Restore
	zipPath := filepath.Join(t.TempDir(), "vault-backup.zip")
	bkRes, err := a.backupVaultToZip(zipPath, recovery.DefaultBackupOptions())
	if err != nil || bkRes.FileCount == 0 {
		t.Fatalf("backupVaultToZip failed: res=%+v err=%v", bkRes, err)
	}
	if err := os.WriteFile(a.vault.AbsPath(tab.FilePath), []byte("corrupted overwrite"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.restoreVaultFromZip(zipPath); err != nil {
		t.Fatalf("restoreVaultFromZip failed: %v", err)
	}
	if cur, _ := os.ReadFile(a.vault.AbsPath(tab.FilePath)); string(cur) != "newer file" {
		t.Fatalf("expected backup restore to recover 'newer file', got %q", string(cur))
	}
}

type m123MockDAV struct {
	mu    sync.Mutex
	files map[string][]byte
}

func newM123MockDAV() *m123MockDAV {
	return &m123MockDAV{files: map[string][]byte{}}
}

func (m *m123MockDAV) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rel := strings.TrimPrefix(r.URL.Path, "/")
	if dec, err := url.PathUnescape(r.URL.EscapedPath()); err == nil {
		rel = strings.TrimPrefix(dec, "/")
	}
	switch r.Method {
	case "PROPFIND":
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="utf-8"?><D:multistatus xmlns:D="DAV:"><D:response><D:href>/%s</D:href><D:propstat><D:prop><D:resourcetype><D:collection/></D:resourcetype></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response></D:multistatus>`, rel)
	case "MKCOL":
		w.WriteHeader(http.StatusCreated)
	case http.MethodGet:
		m.mu.Lock()
		data, ok := m.files[rel]
		m.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	case http.MethodPut:
		body, _ := io.ReadAll(r.Body)
		m.mu.Lock()
		m.files[rel] = body
		m.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
	case http.MethodDelete:
		m.mu.Lock()
		delete(m.files, rel)
		m.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func TestM123SyncHostConfigAndMediaPolicy(t *testing.T) {
	dav := newM123MockDAV()
	srv := httptest.NewServer(dav)
	defer srv.Close()

	// Device A (Desktop): saves config (without leaking password in JSON) + uploads lossless large image + doc
	aDesk, tab := newM123TestApp(t, "---\ntitle: Sync Doc\n---\n\n# 桌面原始记录\n")
	aDesk.syncHTTPClient = srv.Client()

	cfgA := syncHostConfig{
		Enabled:    true,
		BaseURL:    srv.URL,
		Username:   "scientist",
		Password:   "super-secret-webdav-pass",
		PolicyMode: syncengine.PolicyAlwaysOriginal,
		DeviceID:   "desktop-1",
	}
	if err := aDesk.saveSyncConfig(cfgA); err != nil {
		t.Fatalf("saveSyncConfig: %v", err)
	}
	rawCfgJSON, err := os.ReadFile(filepath.Join(aDesk.vault.Root, ".asterlab", "sync_config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(rawCfgJSON), "super-secret-webdav-pass") {
		t.Fatalf("sync_config.json must NEVER contain plaintext password! got:\n%s", string(rawCfgJSON))
	}
	loadedA, err := aDesk.loadSyncConfig()
	if err != nil || loadedA.Password != "super-secret-webdav-pass" {
		t.Fatalf("loadSyncConfig password hydration failed: got %q err=%v", loadedA.Password, err)
	}
	if err := aDesk.testSyncConnection(context.Background()); err != nil {
		t.Fatalf("testSyncConnection failed: %v", err)
	}

	largePNG := makeM123TestPNGBytes(t, 1600, 1200)
	// Pad PNG with an ancillary chunk or test threshold=100 bytes so it exceeds threshold reliably.
	assetRel, err := aDesk.vault.ImportAssetBytes(largePNG, "microscope_raw.png")
	if err != nil {
		t.Fatal(err)
	}
	normAssetRel := strings.TrimPrefix(assetRel, "./")
	repA, err := aDesk.runSyncNow(context.Background())
	if err != nil {
		t.Fatalf("runSyncNow desktop failed: %v", err)
	}
	if len(repA.UploadedFiles) == 0 {
		t.Fatalf("expected desktop sync to upload doc and asset, got %+v", repA)
	}

	// Device B (Mobile threshold policy): threshold=100B -> downloads preview sidecar only, defers original
	bRoot := t.TempDir()
	bMob := &app{shell: &shell.Shell{}, syncHTTPClient: srv.Client()}
	if err := bMob.initVaultWorkspace(bRoot); err != nil {
		t.Fatal(err)
	}
	if err := bMob.saveSyncConfig(syncHostConfig{
		Enabled:        true,
		BaseURL:        srv.URL,
		Username:       "scientist",
		Password:       "super-secret-webdav-pass",
		PolicyMode:     syncengine.PolicyThresholdBytes,
		ThresholdBytes: 100, // Force preview-only for largePNG
		DeviceID:       "mobile-1",
	}); err != nil {
		t.Fatal(err)
	}
	polB := syncengine.DownloadPolicy{
		Mode:           syncengine.PolicyThresholdBytes,
		ThresholdBytes: 100,
		IsMobile:       true,
	}
	bMob.syncPolicyOverride = &polB

	repB, err := bMob.runSyncNow(context.Background())
	if err != nil {
		t.Fatalf("runSyncNow mobile failed: %v", err)
	}
	if len(repB.NeedsOriginalDownload) != 1 || repB.NeedsOriginalDownload[0] != normAssetRel {
		t.Fatalf("expected deferred original %q, got %+v", normAssetRel, repB.NeedsOriginalDownload)
	}
	if _, err := os.Stat(filepath.Join(bRoot, filepath.FromSlash(normAssetRel))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected mobile threshold policy NOT to download full original yet")
	}
	if len(repB.DownloadedPreviews) == 0 {
		t.Fatalf("expected mobile sync to download preview sidecar")
	}

	// On-demand lossless original download
	if err := bMob.fetchAssetOriginalOnDemand(context.Background(), normAssetRel); err != nil {
		t.Fatalf("fetchAssetOriginalOnDemand failed: %v", err)
	}
	gotOrig, err := os.ReadFile(filepath.Join(bRoot, filepath.FromSlash(normAssetRel)))
	if err != nil || !bytes.Equal(gotOrig, largePNG) {
		t.Fatalf("fetched original mismatch (err=%v, len=%d vs %d)", err, len(gotOrig), len(largePNG))
	}

	// Clear local originals cache -> frees local original bytes while keeping preview
	clrRes, err := bMob.clearLocalOriginalsCache()
	if err != nil || clrRes.FilesCleared != 1 {
		t.Fatalf("clearLocalOriginalsCache = (%+v, %v), want 1 cleared", clrRes, err)
	}

	// Concurrent edit conflict -> conflict copy created -> resolveSyncConflict("remote") promotes it
	_ = os.WriteFile(filepath.Join(bRoot, filepath.FromSlash(tab.FilePath)), []byte("---\ntitle: Mobile Edit\n---\n\n# 手机修改\n"), 0o644)
	_ = os.WriteFile(filepath.Join(aDesk.vault.Root, filepath.FromSlash(tab.FilePath)), []byte("---\ntitle: Desktop Edit\n---\n\n# 桌面并发修改\n"), 0o644)
	if _, err := aDesk.runSyncNow(context.Background()); err != nil {
		t.Fatalf("desktop concurrent sync: %v", err)
	}
	repConflict, err := bMob.runSyncNow(context.Background())
	if err != nil {
		t.Fatalf("mobile conflict sync: %v", err)
	}
	var docConflict *syncengine.ConflictEntry
	for i := range repConflict.Conflicts {
		if repConflict.Conflicts[i].RelPath == tab.FilePath {
			docConflict = &repConflict.Conflicts[i]
			break
		}
	}
	if docConflict == nil {
		t.Fatalf("expected conflict entry for %q, got %+v", tab.FilePath, repConflict.Conflicts)
	}
	if err := bMob.resolveSyncConflict(docConflict.RelPath, docConflict.ConflictCopyPath, "remote"); err != nil {
		t.Fatalf("resolveSyncConflict failed: %v", err)
	}
	resolvedBytes, _ := os.ReadFile(filepath.Join(bRoot, filepath.FromSlash(tab.FilePath)))
	if !strings.Contains(string(resolvedBytes), "桌面并发修改") {
		t.Fatalf("expected remote conflict copy promoted, got:\n%s", string(resolvedBytes))
	}
}

func TestM123UserGuideSeedSOP(t *testing.T) {
	vRoot := t.TempDir()
	sh := &shell.Shell{TodayDate: "2026-11-26", SelectedDate: "2026-11-26"}
	a := &app{shell: sh}
	if err := a.initVaultWorkspace(vRoot); err != nil {
		t.Fatalf("initVaultWorkspace: %v", err)
	}
	sh.EnsureTabs()
	a.openOrSeedUserGuideDoc()

	tab := sh.ActiveTab()
	if tab == nil {
		t.Fatal("expected user guide tab to open")
	}
	raw, err := os.ReadFile(a.vault.AbsPath(tab.FilePath))
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	doc, err := asterdoc.Parse(src)
	if err != nil {
		t.Fatalf("seeded user guide parse failed: %v", err)
	}
	for _, want := range []string{"format: 96", "asterlab://pantry?code=", "[^1]", "[^1]:", "mode: dilution", "@plate.sample_wells", "WebDAV"} {
		if !strings.Contains(src, want) {
			t.Fatalf("seeded user guide missing %q", want)
		}
	}
	rep := astercore.AssessDocument(doc, nil)
	for _, d := range rep.Diagnostics {
		if d.Code == "NO_STEPS" || d.Code == "NO_GROUPS" {
			t.Fatalf("unexpected diagnostic %+v", d)
		}
	}
	// Verify Pantry was seeded with near-expiry item for expiry badge showcase
	ledger, err := a.localListPantry()
	if err != nil || len(ledger.Items) == 0 {
		t.Fatalf("expected pantry seeded for @ reagent mention, err=%v len=%d", err, len(ledger.Items))
	}
	var hasNearExpiry bool
	for _, it := range ledger.Items {
		if it.Status == "near_expiry" {
			hasNearExpiry = true
		}
	}
	if !hasNearExpiry {
		t.Fatalf("expected near_expiry reagent in seeded pantry, got %+v", ledger.Items)
	}
}

func TestM123ShellSyncAndBackupCallbackGlue(t *testing.T) {
	vRoot := t.TempDir()
	sh := &shell.Shell{TodayDate: "2026-11-26", SelectedDate: "2026-11-26"}
	a := &app{shell: sh}
	if err := a.initVaultWorkspace(vRoot); err != nil {
		t.Fatalf("initVaultWorkspace: %v", err)
	}
	a.wireRecoveryHost()
	a.wireSyncAndBackupShellCallbacks()

	if sh.OnSaveSyncSettings == nil || sh.OnTestSyncConnection == nil || sh.OnTriggerSyncNow == nil ||
		sh.OnClearOriginalsCache == nil || sh.OnResolveSyncConflict == nil ||
		sh.OnVaultBackupZip == nil || sh.OnVaultRestoreZip == nil {
		t.Fatal("expected all Shell sync and backup callbacks to be wired")
	}

	sh.SyncSettings = shell.SyncSettings{
		Enabled:           true,
		Endpoint:          "https://dav.jianguoyun.com/dav/AsterLab",
		Username:          "lab@example.com",
		Password:          "app-pass",
		ImagePolicy:       shell.SyncImagePolicyThreshold,
		ImageThresholdMB:  5,
		WiFiOnlyOriginals: true,
	}
	sh.OnSaveSyncSettings()

	loaded, err := a.loadSyncConfig()
	if err != nil {
		t.Fatalf("loadSyncConfig after OnSaveSyncSettings: %v", err)
	}
	if !loaded.Enabled || loaded.ThresholdBytes != 5<<20 || !loaded.WiFiOnlyForOriginals {
		t.Fatalf("unexpected persisted sync config via shell callback: %+v", loaded)
	}

	// Verify open_copy & use_remote conflict resolution callbacks through Shell.
	origRel := "冲突验证实验.asterml"
	confRel := "冲突验证实验.sync-conflict-mac-20261126T010000Z.asterml"
	if err := os.WriteFile(filepath.Join(vRoot, origRel), []byte("# 本地原件\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vRoot, confRel), []byte("# 远端并发版本\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.hydrateSyncShellState()
	if len(sh.SyncConflicts) != 1 || sh.SyncState.Status != "conflict" {
		t.Fatalf("expected 1 hydrated conflict, got conflicts=%+v state=%+v", sh.SyncConflicts, sh.SyncState)
	}
	sh.OnResolveSyncConflict(origRel, confRel, "open_copy")
	if sh.ActiveTab() == nil || sh.ActiveTab().FilePath != confRel {
		t.Fatalf("expected open_copy to open %s, got %+v", confRel, sh.ActiveTab())
	}
	sh.OnResolveSyncConflict(origRel, confRel, "use_remote")
	if len(sh.SyncConflicts) != 0 {
		t.Fatalf("expected conflict cleared after use_remote, got %+v", sh.SyncConflicts)
	}
	updated, _ := os.ReadFile(filepath.Join(vRoot, origRel))
	if !strings.Contains(string(updated), "远端并发版本") {
		t.Fatalf("expected use_remote to promote remote content, got %s", string(updated))
	}
}

func TestSyncHostConfigPersistenceAndEffectiveWebDAVURL(t *testing.T) {
	if got := effectiveWebDAVBaseURL("https://dav.jianguoyun.com/dav", ""); got != "https://dav.jianguoyun.com/dav" {
		t.Fatalf("empty remotePath mutated baseURL: %q", got)
	}
	if got := effectiveWebDAVBaseURL("https://dav.jianguoyun.com/dav", "/AsterLab"); got != "https://dav.jianguoyun.com/dav/AsterLab/" {
		t.Fatalf("non-empty remotePath not joined: %q", got)
	}
	if got := effectiveWebDAVBaseURL("https://dav.jianguoyun.com/dav/AsterLab", "/AsterLab"); got != "https://dav.jianguoyun.com/dav/AsterLab" {
		t.Fatalf("duplicate /AsterLab doubled: %q", got)
	}

	vRoot := t.TempDir()
	sh := &shell.Shell{TodayDate: "2026-11-27", SelectedDate: "2026-11-27"}
	a := &app{shell: sh}
	if err := a.initVaultWorkspace(vRoot); err != nil {
		t.Fatal(err)
	}
	a.wireSyncAndBackupShellCallbacks()

	sh.SyncSettings = shell.SyncSettings{
		Provider:          shell.SyncProviderOneDrive,
		Enabled:           true,
		RemotePath:        "/LabVault-B",
		AutoIntervalMin:   15,
		OAuthClientID:     "azure-app-123",
		ImagePolicy:       shell.SyncImagePolicyThreshold,
		ImageThresholdMB:  2,
		WiFiOnlyOriginals: true,
	}
	sh.OnSaveSyncSettings()

	// Fresh app reading persisted config should hydrate RemotePath, AutoIntervalMin, Provider, and OAuthClientID.
	sh2 := &shell.Shell{TodayDate: "2026-11-27", SelectedDate: "2026-11-27"}
	a2 := &app{shell: sh2}
	if err := a2.initVaultWorkspace(vRoot); err != nil {
		t.Fatal(err)
	}
	a2.wireSyncAndBackupShellCallbacks()
	if sh2.SyncSettings.Provider != shell.SyncProviderOneDrive ||
		sh2.SyncSettings.RemotePath != "/LabVault-B" ||
		sh2.SyncSettings.AutoIntervalMin != 15 ||
		sh2.SyncSettings.OAuthClientID != "azure-app-123" {
		t.Fatalf("sync settings did not round-trip: %+v", sh2.SyncSettings)
	}
}

func TestSwitchVaultRootErrValidationAndToast(t *testing.T) {
	vRoot1 := t.TempDir()
	vRoot2 := t.TempDir()
	sh := &shell.Shell{TodayDate: "2026-11-27", SelectedDate: "2026-11-27"}
	a := &app{shell: sh}
	if err := a.initVaultWorkspace(vRoot1); err != nil {
		t.Fatal(err)
	}
	a.wireRecoveryHost()
	a.wireSyncAndBackupShellCallbacks()

	if err := a.switchVaultRootErr("   "); err == nil {
		t.Fatalf("expected error on empty vault path")
	}
	// Passing a file path as vault directory fails writable directory probe without tearing down active vault.
	blockerFile := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blockerFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.switchVaultRootErr(blockerFile); err == nil {
		t.Fatalf("expected writable probe error when path is a regular file")
	}
	if a.vault == nil || filepath.Clean(a.vault.Root) != filepath.Clean(vRoot1) {
		t.Fatalf("expected original vault preserved after failed switch, got %v", a.vault)
	}

	// Valid switch succeeds, updates shell VaultPathInput, ensures tab, and pushes success toast.
	if err := a.switchVaultRootErr(vRoot2); err != nil {
		t.Fatalf("switchVaultRootErr valid path failed: %v", err)
	}
	if filepath.Clean(sh.VaultPathInput) != filepath.Clean(vRoot2) {
		t.Fatalf("expected VaultPathInput=%s, got %s", vRoot2, sh.VaultPathInput)
	}
	if sh.ActiveTab() == nil {
		t.Fatalf("expected ActiveTab ensured after vault switch")
	}
}

