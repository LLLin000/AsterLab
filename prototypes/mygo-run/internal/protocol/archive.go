// Package protocol archive builds the 终态归档: the frozen, exportable record
// of one Run. The archive renders Core's single read-only RecordProjection
// (parameters, timeline, image matrix, timers, notes, archive state and
// issues) plus the研判 prose with its dynamic references flattened, so the
// exported package stands alone without the app. It performs no derivation:
// plan slots, groups and counts come from the Core projection; Go only
// adapts them for Markdown, PDF and the checksum bundle.
package protocol

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"asterlab-mygo-prototype/internal/notebook/record"

	"github.com/jung-kurt/gofpdf"
)

// Archive is the exportable end state of one Run's record.
type Archive struct {
	Projection record.Projection
	Tasks      []record.TaskStatus
	Prose      string
	// SourceRefs remembers the dynamic references the prose cited, so the
	// representative matrix slice follows what the author actually referenced.
	SourceRefs []record.MatrixRef
	ExportedAt time.Time
}

// BuildArchive assembles the archive from the Core projection and the current
// prose. Dynamic `@matrix(...)` references are expanded against the
// projection; prose without references passes through untouched. A legacy
// prose-signature trailer (pre-Core-sign HTML comment) is stripped for
// display: it is history-only, never authority.
func BuildArchive(proj record.Projection, tasks []record.TaskStatus, prose string, now time.Time) Archive {
	return Archive{
		Projection: proj,
		Tasks:      tasks,
		Prose:      record.ExpandMatrixRefs(prose, proj),
		SourceRefs: record.FindMatrixRefs(prose),
		ExportedAt: now,
	}
}

// Counts returns the one shared counting of the archived projection.
func (arch Archive) Counts() record.Counts {
	return record.Summarize(arch.Projection, arch.Tasks)
}

// Markdown renders the archive package: provenance header, shared counts,
// planned-vs-actual comparison, timeline, timers, representative matrix
// slice, expanded prose, and the Core sign-off manifest.
func (arch Archive) Markdown() string {
	proj := arch.Projection
	counts := arch.Counts()
	var sb strings.Builder
	sb.WriteString("# 实验记录归档\n\n")
	sb.WriteString(fmt.Sprintf("- Run：%s\n", displayValue(proj.RunID)))
	sb.WriteString(fmt.Sprintf("- 设计：%s · 冻结 rev %d\n", displayValue(proj.Design.Title), proj.Design.DesignRevision))
	sb.WriteString(fmt.Sprintf("- 状态：%s\n", record.ArchiveStatusWord(proj.Archive.Status)))
	if arch.ExportedAt.IsZero() {
		sb.WriteString("- 导出时间：未导出（预览）\n")
	} else {
		sb.WriteString(fmt.Sprintf("- 导出时间：%s\n", arch.ExportedAt.UTC().Format(time.RFC3339)))
	}
	if sig := proj.Archive.Signature; sig != nil {
		sb.WriteString(fmt.Sprintf("- 签核：%s · %s\n", sig.SignedBy, sig.SignedAtUTC))
		if sig.Statement != nil && strings.TrimSpace(*sig.Statement) != "" {
			sb.WriteString(fmt.Sprintf("- 签核说明：%s\n", strings.TrimSpace(*sig.Statement)))
		}
	} else {
		sb.WriteString("- 签核：未签核\n")
	}
	sb.WriteString(fmt.Sprintf("- %s\n\n", record.Headline(counts)))
	sb.WriteString("## 计划任务\n\n")
	if len(arch.Tasks) == 0 {
		sb.WriteString("无冻结任务。\n\n")
	}
	for _, task := range arch.Tasks {
		title := strings.TrimSpace(task.Title)
		if title == "" {
			title = strings.TrimSpace(task.Key)
		}
		line := "- " + title
		if strings.TrimSpace(task.Timepoint) != "" {
			line += " · " + task.Timepoint
		}
		line += " · " + record.TaskStatusWord(task.Status)
		sb.WriteString(line + "\n")
	}
	sb.WriteString("\n## 计划 vs 实际\n\n")
	if len(proj.Parameters) == 0 {
		sb.WriteString("无计划参数。\n\n")
	}
	for _, group := range record.GroupParameters(proj.Parameters) {
		sb.WriteString(fmt.Sprintf("### %s\n\n", group.Title))
		sb.WriteString("| 参数 | 计划 | 实际 | 状态 |\n| --- | --- | --- | --- |\n")
		for _, row := range group.Rows {
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", row.Label, row.Planned, row.Actual, row.Status))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("## 时间线\n\n")
	sb.WriteString(fmt.Sprintf("- 记录事件 %d 条 · 失败 %d 条 · 重试 %d 条 · 条件跳过 %d 次\n",
		len(proj.Timeline.Events), len(proj.Timeline.Failures), len(proj.Timeline.Retries), proj.Timeline.SkippedByRule))
	if proj.Timeline.FirstAt != nil {
		sb.WriteString(fmt.Sprintf("- 首次记录：%s\n", *proj.Timeline.FirstAt))
	}
	if proj.Timeline.LastAt != nil {
		sb.WriteString(fmt.Sprintf("- 末次记录：%s\n", *proj.Timeline.LastAt))
	}
	for _, event := range proj.Timeline.Events {
		if event.Deviation == nil || strings.TrimSpace(*event.Deviation) == "" {
			continue
		}
		sb.WriteString(fmt.Sprintf("- 偏差：%s\n", strings.TrimSpace(*event.Deviation)))
	}
	sb.WriteString("\n## 计时\n\n")
	if len(proj.Timers) == 0 {
		sb.WriteString("无计时记录。\n\n")
	}
	for _, timer := range proj.Timers {
		sb.WriteString("- " + timerLine(timer) + "\n")
	}
	sb.WriteString("\n## 代表视野矩阵\n\n")
	tps := record.MatrixTimepoints(proj)
	if len(record.MatrixWells(proj)) == 0 || len(tps) == 0 {
		sb.WriteString("无实测孔位数据。\n\n")
	} else {
		ref := record.MatrixRef{All: true}
		if len(arch.SourceRefs) > 0 {
			ref = arch.SourceRefs[0]
		}
		sb.WriteString(record.RenderMatrixTable(proj, ref))
		sb.WriteString("\n")
		for _, cell := range record.MatrixCells(proj) {
			if cell.Filled > 0 {
				sb.WriteString(fmt.Sprintf("- %s · %s：图 %d/%d\n",
					cell.Well, cell.Timepoint, cell.Filled, cell.Planned))
			}
		}
		sb.WriteString("\n")
	}
	if ids := proj.Images.UnlocatedAssetIDs; len(ids) > 0 {
		sb.WriteString(fmt.Sprintf("未定位图 %d 张（无孔位坐标，不归入视野槽）：\n\n", len(ids)))
		for _, id := range ids {
			sb.WriteString("- " + id + "\n")
		}
		sb.WriteString("\n")
	}
	if ids := proj.Images.UnplannedAssetIDs; len(ids) > 0 {
		sb.WriteString(fmt.Sprintf("计划外图 %d 张（超出计划视野或无计划行）：\n\n", len(ids)))
		for _, id := range ids {
			sb.WriteString("- " + id + "\n")
		}
		sb.WriteString("\n")
	}
	for _, issue := range proj.Issues {
		sb.WriteString(fmt.Sprintf("> 缺项：%s（%s）\n\n", issue.Message, issue.Code))
	}
	sb.WriteString("## 研判记录\n\n")
	if strings.TrimSpace(record.StripLegacySignature(arch.Prose)) == "" {
		sb.WriteString("（无研判正文）\n\n")
	} else {
		sb.WriteString(strings.TrimSpace(record.StripLegacySignature(arch.Prose)) + "\n\n")
	}
	sb.WriteString("## 签核清单\n\n")
	if proj.Archive.Manifest == nil {
		sb.WriteString("未签核归档：签核在 Notebook 图板完成，归档包随之锁定。\n")
	} else {
		manifest := proj.Archive.Manifest
		byKind := map[string]int{}
		for _, entry := range manifest.Entries {
			byKind[entry.Kind]++
		}
		sb.WriteString(fmt.Sprintf("清单版本 %d · 成员 %d 个（资产 %d · 笔记 %d · 投影 %d）\n\n",
			manifest.SchemaVersion, len(manifest.Entries),
			byKind["asset"], byKind["note"], byKind["projection"]))
		sb.WriteString(fmt.Sprintf("- 清单摘要：%s\n", manifest.ManifestSHA256))
		sb.WriteString(fmt.Sprintf("- 生成时间：%s\n", manifest.GeneratedAtUTC))
	}
	return sb.String()
}

// timerLine renders one Core timer observation with its measured elapsed
// time at the projection watermark.
func timerLine(timer record.TimerObserve) string {
	var sb strings.Builder
	sb.WriteString(timer.TimerID + " · " + timer.TaskKey + " · " + timer.Status)
	if timer.PlannedS != nil {
		fmt.Fprintf(&sb, " · 计划 %ds", *timer.PlannedS)
	}
	if timer.ElapsedS != nil {
		fmt.Fprintf(&sb, " · 实测 %ds", *timer.ElapsedS)
	} else {
		sb.WriteString(" · 实测 —")
	}
	return sb.String()
}

func displayValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "未记录"
	}
	return value
}

// recordPDFFontCandidates copies the working set from internal/asterdoc/pdf.go
// (chineseFontCandidates): only single-font .ttf files, never .ttc
// collections — gofpdf cannot parse TTC (e.g. simsun.ttc), and a TTC-shaped
// candidate poisons the family without an immediate error. protocol cannot
// import asterdoc for the list (asterdoc/view.go imports protocol), so the
// copy is deliberate; keep the two in sync. Unlike the frozen-document
// export, a missing font is not fatal here: the PDF still builds with
// Helvetica (Chinese glyphs degrade) so the image board and the parameter
// facts always export. Windows Gate machines carry simhei.ttf.
var recordPDFFontCandidates = []string{
	"C:/Windows/Fonts/simhei.ttf",
	"C:/Windows/Fonts/simsunb.ttf",
	"C:/Windows/Fonts/NotoSansSC-VF.ttf",
	"/usr/share/fonts/truetype/droid/DroidSansFallbackFull.ttf",
}

// usableFontFile reports whether data looks like a single-font file gofpdf
// can parse: a .ttf/.otf suffix plus a TrueType/OpenType sfnt magic. This
// rejects .ttc collections ('ttcf') and corrupt bytes before they reach
// gofpdf, where a bad registration surfaces only later as a sticky
// 'undefined font' failure far from the cause.
func usableFontFile(path string, data []byte) bool {
	lower := strings.ToLower(path)
	if !strings.HasSuffix(lower, ".ttf") && !strings.HasSuffix(lower, ".otf") {
		return false
	}
	if len(data) < 4 {
		return false
	}
	magic := string(data[:4])
	return magic == "\x00\x01\x00\x00" || magic == "OTTO" || magic == "true" || magic == "typ1"
}

// ImageLoader returns PNG bytes for one placed matrix image. The caller wires
// it to assets.Roots.Resolve plus a decode/re-encode step (TIFF masters
// decode through the shared preview decoder, gofpdf embeds PNG). Any error
// aborts the export with the asset named: images are never silently dropped.
type ImageLoader func(record.PlacedImage) ([]byte, error)

// AssetBytesLoader returns the original bytes of one archived member named by
// its manifest logical path. The bundle builder verifies every byte against
// the manifest: a missing or unreadable member fails loudly instead of
// shipping a fake-success package.
type AssetBytesLoader func(logicalPath string) ([]byte, error)

// WriteBundle assembles the archive package: the rendered PDF and Markdown,
// Core's exact manifest text, every manifest asset and note verified
// byte-for-byte, nothing else. Manifest entries of kind projection describe
// Core-internal bytes the bundle cannot reproduce, so they are listed but
// not verified; every asset and note entry must match, or the whole export
// aborts naming the member. Logical paths are confined to the bundle:
// absolute paths and `..` segments are rejected before any file is read.
func (arch Archive) WriteBundle(w io.Writer, manifestJSON string, images ImageLoader, files AssetBytesLoader) error {
	if strings.TrimSpace(manifestJSON) == "" {
		return fmt.Errorf("归档包需要签核清单：先完成签核归档，再导出归档包")
	}
	var manifest record.Manifest
	if err := json.Unmarshal([]byte(manifestJSON), &manifest); err != nil {
		return fmt.Errorf("签核清单无法解析：%w", err)
	}
	projection := arch.Projection
	manifestByPath := map[string]record.ManifestEntry{}
	for _, entry := range manifest.Entries {
		if err := checkBundlePath(entry.LogicalPath); err != nil {
			return fmt.Errorf("签核清单含非法路径 %q：%w", entry.LogicalPath, err)
		}
		manifestByPath[entry.LogicalPath] = entry
	}
	if err := verifyManifestDigest(manifest); err != nil {
		return err
	}
	zipWriter := zip.NewWriter(w)
	addFile := func(name string, data []byte) error {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.Modified = bundleStamp(arch.ExportedAt)
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}
		_, err = writer.Write(data)
		return err
	}
	var pdfBuf bytes.Buffer
	if err := arch.PDF(&pdfBuf, images); err != nil {
		return err
	}
	if err := addFile("record.pdf", pdfBuf.Bytes()); err != nil {
		return err
	}
	if err := addFile("record.md", []byte(arch.Markdown())); err != nil {
		return err
	}
	if err := addFile("manifest.json", []byte(manifestJSON)); err != nil {
		return err
	}
	bundled := map[string]bool{"record.pdf": true, "record.md": true, "manifest.json": true}
	for logicalPath, entry := range manifestByPath {
		if entry.Kind != "asset" && entry.Kind != "note" {
			continue
		}
		name := "assets/" + logicalPath
		if entry.Kind == "note" {
			name = logicalPath
		}
		if bundled[name] {
			return fmt.Errorf("归档包路径冲突：%q", name)
		}
		bundled[name] = true
		var data []byte
		var err error
		if entry.Kind == "note" {
			data, err = noteBytes(projection, logicalPath)
		} else {
			if files == nil {
				return fmt.Errorf("归档资产 %q：未配置文件加载器", logicalPath)
			}
			data, err = files(logicalPath)
		}
		if err != nil {
			return fmt.Errorf("归档成员 %q 不可读：%w", logicalPath, err)
		}
		if err := checkEntryBytes(entry, data); err != nil {
			return err
		}
		if err := addFile(name, data); err != nil {
			return err
		}
	}
	if err := zipWriter.Close(); err != nil {
		return err
	}
	return nil
}

// checkBundlePath confines a manifest logical path to the bundle: only
// relative `root/relative` or `notes/<id>.md` style paths, never absolute,
// never escaping with `..`.
func checkBundlePath(logicalPath string) error {
	if strings.TrimSpace(logicalPath) == "" {
		return fmt.Errorf("空路径")
	}
	if path.IsAbs(logicalPath) {
		return fmt.Errorf("绝对路径")
	}
	cleaned := path.Clean(logicalPath)
	if cleaned != logicalPath || cleaned == "." || strings.HasPrefix(cleaned, "../") {
		return fmt.Errorf("非法路径")
	}
	for _, part := range strings.Split(cleaned, "/") {
		if part == ".." || part == "" {
			return fmt.Errorf("非法路径段")
		}
	}
	return nil
}

// verifyManifestDigest recomputes the canonical manifest digest
// (`"<sha256>  <logical_path>\n"` per entry, sorted by path) and compares it
// to the manifest's own claim.
func verifyManifestDigest(manifest record.Manifest) error {
	paths := make([]string, 0, len(manifest.Entries))
	byPath := map[string]record.ManifestEntry{}
	for _, entry := range manifest.Entries {
		paths = append(paths, entry.LogicalPath)
		byPath[entry.LogicalPath] = entry
	}
	sort.Strings(paths)
	hasher := sha256.New()
	for _, logicalPath := range paths {
		fmt.Fprintf(hasher, "%s  %s\n", strings.ToLower(byPath[logicalPath].SHA256), logicalPath)
	}
	digest := hex.EncodeToString(hasher.Sum(nil))
	if digest != strings.ToLower(manifest.ManifestSHA256) {
		return fmt.Errorf("签核清单摘要不一致：清单声称 %s，实际 %s", manifest.ManifestSHA256, digest)
	}
	return nil
}

// noteBytes renders one manifest note entry from the projection's stored
// notes. The bytes must hash exactly as Core recorded them.
func noteBytes(proj record.Projection, logicalPath string) ([]byte, error) {
	if !strings.HasPrefix(logicalPath, "notes/") || !strings.HasSuffix(logicalPath, ".md") {
		return nil, fmt.Errorf("不是笔记路径")
	}
	id := strings.TrimSuffix(strings.TrimPrefix(logicalPath, "notes/"), ".md")
	for _, note := range proj.Notes {
		if note.NoteID == id {
			return []byte(note.Markdown), nil
		}
	}
	return nil, fmt.Errorf("投影中没有该笔记")
}

// checkEntryBytes verifies one bundled member against its manifest entry:
// SHA256 first, then size when the manifest records one.
func checkEntryBytes(entry record.ManifestEntry, data []byte) error {
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != strings.ToLower(entry.SHA256) {
		return fmt.Errorf("归档成员 %q 校验失败：内容与清单 SHA256 不一致", entry.LogicalPath)
	}
	if entry.Size != nil && *entry.Size != uint64(len(data)) {
		return fmt.Errorf("归档成员 %q 长度不一致：清单 %d，实际 %d", entry.LogicalPath, *entry.Size, len(data))
	}
	return nil
}

// bundleStamp pins every zip member to the export instant (or a fixed stamp
// for previews) so identical facts produce identical packages.
func bundleStamp(exportedAt time.Time) time.Time {
	if exportedAt.IsZero() {
		return time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	return exportedAt.UTC()
}

// PDF renders the same end state as Markdown plus an embedded image board
// from the same projection: provenance header, planned-vs-actual comparison,
// timeline, timers, expanded prose, the representative Well x Timepoint x FOV
// images, and the Core sign-off manifest. Output is deterministic for
// identical inputs (fixed PDF stamps; pass the same ExportedAt): only the
// file bytes change when facts change.
func (arch Archive) PDF(w io.Writer, load ImageLoader) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetCreationDate(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	pdf.SetModificationDate(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	pdf.SetMargins(16, 16, 16)
	pdf.SetAutoPageBreak(true, 16)
	font := "Helvetica"
	for _, path := range recordPDFFontCandidates {
		data, err := os.ReadFile(path)
		if err != nil || !usableFontFile(path, data) {
			continue
		}
		// Single-style registration only: headings are sized, not bolded,
		// so a fontless machine falls back to core Helvetica (which always
		// defines "") with no dangling family/weight reference.
		pdf.AddUTF8FontFromBytes("RecordCJK", "", data)
		if err := pdf.Error(); err != nil {
			return fmt.Errorf("record PDF: load CJK font %q: %w", path, err)
		}
		font = "RecordCJK"
		break
	}
	proj := arch.Projection
	counts := arch.Counts()
	pdf.SetTitle("Run record archive "+proj.RunID, true)
	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont(font, "", 8)
		pdf.CellFormat(0, 8, fmt.Sprintf("Page %d", pdf.PageNo()), "", 0, "C", false, 0, "")
	})
	h1 := func(text string) {
		pdf.SetFont(font, "", 15)
		pdf.MultiCell(0, 8, text, "", "", false)
		pdf.Ln(1)
	}
	h2 := func(text string) {
		pdf.SetFont(font, "", 12)
		pdf.MultiCell(0, 7, text, "", "", false)
	}
	body := func(text string) {
		pdf.SetFont(font, "", 9)
		pdf.MultiCell(0, 5, text, "", "", false)
	}
	pdf.AddPage()
	h1("实验记录归档")
	stamp := "未导出（预览）"
	if !arch.ExportedAt.IsZero() {
		stamp = arch.ExportedAt.UTC().Format(time.RFC3339)
	}
	for _, line := range []string{
		"Run：" + displayValue(proj.RunID),
		"设计：" + displayValue(proj.Design.Title) + " · 冻结 rev " + fmt.Sprint(proj.Design.DesignRevision),
		"状态：" + record.ArchiveStatusWord(proj.Archive.Status),
		"导出时间：" + stamp,
	} {
		body(line)
	}
	if sig := proj.Archive.Signature; sig != nil {
		body("签核：" + sig.SignedBy + " · " + sig.SignedAtUTC)
		if sig.Statement != nil && strings.TrimSpace(*sig.Statement) != "" {
			body("签核说明：" + strings.TrimSpace(*sig.Statement))
		}
	} else {
		body("签核：未签核")
	}
	body(record.Headline(counts))
	pdf.Ln(2)
	h2("计划任务")
	if len(arch.Tasks) == 0 {
		body("无冻结任务。")
	}
	for _, task := range arch.Tasks {
		title := strings.TrimSpace(task.Title)
		if title == "" {
			title = strings.TrimSpace(task.Key)
		}
		line := title
		if strings.TrimSpace(task.Timepoint) != "" {
			line += " · " + task.Timepoint
		}
		body(line + " · " + record.TaskStatusWord(task.Status))
	}
	h2("计划 vs 实际")
	if len(proj.Parameters) == 0 {
		body("无计划参数。")
	}
	for _, group := range record.GroupParameters(proj.Parameters) {
		pdf.SetFont(font, "", 10)
		pdf.MultiCell(0, 6, group.Title, "", "", false)
		for _, row := range group.Rows {
			body(fmt.Sprintf("| %s | %s | %s | %s |", row.Label, row.Planned, row.Actual, row.Status))
		}
		pdf.Ln(1)
	}
	h2("时间线")
	body(fmt.Sprintf("记录事件 %d 条 · 失败 %d 条 · 重试 %d 条 · 条件跳过 %d 次",
		len(proj.Timeline.Events), len(proj.Timeline.Failures), len(proj.Timeline.Retries), proj.Timeline.SkippedByRule))
	h2("计时")
	if len(proj.Timers) == 0 {
		body("无计时记录。")
	}
	for _, timer := range proj.Timers {
		body(timerLine(timer))
	}
	h2("代表视野矩阵")
	fovSlots := int(proj.Images.FOVPerWell)
	if fovSlots < 1 {
		fovSlots = 1
	}
	ref := record.MatrixRef{All: true}
	if len(arch.SourceRefs) > 0 {
		ref = arch.SourceRefs[0]
	}
	cells, _ := record.ResolveMatrixRef(proj, ref)
	if len(cells) == 0 {
		body("无实测孔位数据。")
	}
	for _, cell := range cells {
		body(fmt.Sprintf("%s · %s：图 %d/%d", cell.Well, cell.Timepoint, cell.Filled, cell.Planned))
		for fov := 1; fov <= fovSlots; fov++ {
			img := record.SlotImage(proj, cell.Well, cell.Timepoint, fov)
			if img == nil {
				continue
			}
			body(fmt.Sprintf("%s · %s · FOV %d — %s", cell.Well, cell.Timepoint, fov, img.AssetID))
			if load == nil {
				return fmt.Errorf("record PDF image %q (%s · %s · FOV %d): no image loader wired", img.AssetID, cell.Well, cell.Timepoint, fov)
			}
			data, err := load(*img)
			if err != nil {
				return fmt.Errorf("record PDF image %q (%s · %s · FOV %d) unavailable: %w", img.AssetID, cell.Well, cell.Timepoint, fov, err)
			}
			name := "record-" + img.AssetID
			pdf.RegisterImageReader(name, "PNG", bytes.NewReader(data))
			if err := pdf.Error(); err != nil {
				return fmt.Errorf("record PDF image %q (%s · %s · FOV %d) rejected: %w", img.AssetID, cell.Well, cell.Timepoint, fov, err)
			}
			cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
			imgW, imgH := 150.0, 100.0
			if err == nil && cfg.Width > 0 && cfg.Height > 0 {
				imgW = 150.0
				imgH = 150.0 * float64(cfg.Height) / float64(cfg.Width)
				if imgH > 90 {
					imgH = 90
					imgW = 90 * float64(cfg.Height) / float64(cfg.Width)
				}
			}
			if pdf.GetY()+imgH+4 > 297-16 {
				pdf.AddPage()
			}
			x := 16.0
			if imgW < 178 {
				x = 16 + (178-imgW)/2
			}
			pdf.Image(name, x, pdf.GetY(), imgW, imgH, false, "", 0, "")
			if err := pdf.Error(); err != nil {
				return fmt.Errorf("record PDF image %q (%s · %s · FOV %d) embed failed: %w", img.AssetID, cell.Well, cell.Timepoint, fov, err)
			}
			pdf.Ln(imgH + 2)
		}
	}
	if ids := proj.Images.UnlocatedAssetIDs; len(ids) > 0 {
		h2("未定位图")
		for _, id := range ids {
			body(id)
		}
	}
	if ids := proj.Images.UnplannedAssetIDs; len(ids) > 0 {
		h2("计划外图")
		for _, id := range ids {
			body(id)
		}
	}
	h2("研判记录")
	for _, line := range strings.Split(strings.TrimSpace(record.StripLegacySignature(arch.Prose)), "\n") {
		if strings.TrimSpace(line) == "" {
			pdf.Ln(2)
			continue
		}
		body(line)
	}
	h2("签核清单")
	if proj.Archive.Manifest == nil {
		body("未签核归档。")
	} else {
		manifest := proj.Archive.Manifest
		body(fmt.Sprintf("清单版本 %d · 成员 %d 个 · 摘要 %s", manifest.SchemaVersion, len(manifest.Entries), manifest.ManifestSHA256))
	}
	return pdf.Output(w)
}
