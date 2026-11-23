package main

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/astercmd"
	"asterlab-mygo-prototype/internal/astercore"
	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/shell"
)

// TestScenario_AllFourModernSOPWorkflowsEndToEnd exercises the complete 9-stage
// wet-lab lifecycle (Create -> Edit -> Plate/Group -> Suggest/SciGraph -> LabMode
// -> Record/Photo -> Restart -> External Disk Sync -> Validate/Assess/PDF Export)
// across all 4 built-in modern #@ SOP templates (HUVEC, CFU, Inhibition Zone, RT-qPCR).
func TestScenario_AllFourModernSOPWorkflowsEndToEnd(t *testing.T) {
	sops := []struct {
		name     string
		relPath  string
		source   string
		wantText string
	}{
		{
			name:     "HUVEC",
			relPath:  "Projects/HUVEC-成管.asterml",
			source:   huvecAsterDoc,
			wantText: "HUVEC 水凝胶体外成管测定",
		},
		{
			name:     "CFU",
			relPath:  "Projects/CFU-菌落计数.asterml",
			source:   cfuAsterDoc,
			wantText: "梯度稀释涂板菌落计数（CFU）",
		},
		{
			name:     "InhibitionZone",
			relPath:  "Projects/抑菌圈测定.asterml",
			source:   inhibitionZoneAsterDoc,
			wantText: "四象限滤纸片抑菌圈测定",
		},
		{
			name:     "RTqPCR",
			relPath:  "Projects/RT-qPCR-多靶点.asterml",
			source:   rtQpcrAsterDoc,
			wantText: "RT-qPCR 多靶点相对定量实验",
		},
	}

	vaultRoot := filepath.Join(t.TempDir(), "ScenarioVault")
	if err := os.MkdirAll(filepath.Join(vaultRoot, "Projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(vaultRoot, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Seed a tiny valid 1x1 PNG asset for inline relation attachment testing.
	tinyPNG := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
		0x00, 0x03, 0x01, 0x01, 0x00, 0x18, 0xdd, 0x8d,
		0xb0, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
		0x44, 0xae, 0x42, 0x60, 0x82,
	}
	if err := os.WriteFile(filepath.Join(vaultRoot, "assets", "obs-a1.png"), tinyPNG, 0o644); err != nil {
		t.Fatal(err)
	}

	// Stage 1: Create all 4 modern SOP documents in the Vault.
	for _, sop := range sops {
		abs := filepath.Join(vaultRoot, filepath.FromSlash(sop.relPath))
		stamped := asterdoc.StampScheduledDate(sop.source, "2026-11-24")
		if err := asterdoc.WriteFileAtomic(abs, []byte(stamped)); err != nil {
			t.Fatalf("%s seed write: %v", sop.name, err)
		}
	}

	a, sh := newWiredFSMTestApp(t, vaultRoot)
	st := ui.NewTester(func(c *ui.Context) {
		sh.RenderSplitView(c)
	}, 1360, 880)
	assertBoundedFrame(t, st, "scenario-boot")

	for _, sop := range sops {
		sop := sop
		t.Run(sop.name, func(t *testing.T) {
			abs := filepath.Join(vaultRoot, filepath.FromSlash(sop.relPath))

			// Stage 2: Open in Edit mode & verify rendered title + #@ step tree
			sh.OpenFileInPane(sh.ActivePaneID, sop.relPath, sop.name, shell.ModeEdit, false)
			assertBoundedFrame(t, st, sop.name+"-edit-open")
			if !st.HasText(sop.wantText) {
				t.Fatalf("%s missing title text %q in GUI frame", sop.name, sop.wantText)
			}

			raw, err := os.ReadFile(abs)
			if err != nil {
				t.Fatal(err)
			}
			src := string(raw)
			doc, err := asterdoc.Parse(src)
			if err != nil {
				t.Fatalf("%s initial Parse failed: %v", sop.name, err)
			}
			tree := asterdoc.ParseStepTree(src, doc)
			if tree == nil || len(tree.Roots) < 3 {
				t.Fatalf("%s expected >= 3 #@ steps, got %+v", sop.name, tree)
			}

			// Stage 3 & 4: Semantic batch edit (append note + embed photo linked to first plate if present) + offline NL suggest
			var targets []string
			for _, m := range doc.Modules {
				if m.Kind == "plate" {
					targets = append(targets, "@"+m.ID+":A1")
					break
				}
			}
			embedArgs := map[string]any{
				"asset":     "./assets/obs-a1.png",
				"timepoint": "24h",
				"fov":       1,
				"role":      "obs",
			}
			if len(targets) > 0 {
				embedArgs["targets"] = targets
			}
			batchRes := astercmd.ExecuteBatch(src, astercmd.BatchRequest{
				ExpectedHash: astercmd.DocumentHash(src),
				Mode:         astercmd.ModeEdit,
				Operations: []astercmd.Operation{
					{CommandID: "doc.append_note", Args: map[string]any{"text": "场景测试备注：已完成 " + sop.name + " 关键条件核对。"}},
					{CommandID: "asset.embed", Args: embedArgs},
				},
			})
			if !batchRes.OK {
				t.Fatalf("%s semantic batch failed: %+v", sop.name, batchRes.Error)
			}
			if err := asterdoc.WriteFileAtomic(abs, []byte(batchRes.NextSource)); err != nil {
				t.Fatal(err)
			}
			updatedDoc, err := asterdoc.Parse(batchRes.NextSource)
			if err != nil {
				t.Fatalf("%s post-edit Parse failed: %v", sop.name, err)
			}
			if len(targets) > 0 {
				rels := asterdoc.ResolveRelations(updatedDoc)
				if len(rels) == 0 || len(rels[0].Contexts) == 0 || !rels[0].Contexts[0].Valid {
					t.Fatalf("%s expected valid plate relation on embedded photo, got %+v", sop.name, rels)
				}
			}
			// Built-in offline AI suggestion report must compile cleanly inside its target document (preserving SciGraph plate refs)
			sugRep := asterdoc.SuggestDocumentModules(updatedDoc)
			for _, sg := range sugRep.Suggestions {
				if _, pErr := asterdoc.Parse(updatedDoc.Source + "\n\n" + sg.YAML + "\n"); pErr != nil {
					t.Fatalf("%s suggestion YAML invalid when inserted into document: %v\n%s", sop.name, pErr, sg.YAML)
				}
			}

			// Stage 5 & 6: Switch to Lab Mode (anti-accidental-edit) and verify frame renders cleanly
			sh.SetActiveTabMode(shell.ModeLab)
			assertBoundedFrame(t, st, sop.name+"-lab-mode")
			assertGlobalShellInvariants(t, sh, sop.name+"-lab-mode")

			// Stage 8: External disk modification + live disk watcher sync
			extNote := "\n\n> 外部同步标记-" + sop.name + "\n"
			if err := asterdoc.WriteFileAtomic(abs, []byte(batchRes.NextSource+extNote)); err != nil {
				t.Fatal(err)
			}
			future := time.Now().Add(3 * time.Second)
			_ = os.Chtimes(abs, future, future)
			a.syncVaultDiskChangesNow()
			sh.SetActiveTabMode(shell.ModeEdit)
			assertBoundedFrame(t, st, sop.name+"-after-external-sync")
			ld, ldErr := a.docStore.Open(sop.relPath)
			if ldErr != nil || ld == nil || !strings.Contains(ld.Source, "外部同步标记-"+sop.name) {
				t.Fatalf("%s live disk watcher did not refresh external edit (err=%v)", sop.name, ldErr)
			}

			// Stage 9: Assess + Analyze + PDF Export verification
			finalDoc, err := asterdoc.Parse(ld.Source)
			if err != nil {
				t.Fatalf("%s final Parse failed: %v", sop.name, err)
			}
			assess := astercore.AssessDocument(finalDoc, nil)
			if assess.Blocking > 0 {
				t.Fatalf("%s unexpected blocking assessment issues: %+v", sop.name, assess.Diagnostics)
			}
			_ = astercore.AnalyzeDocument(finalDoc, sop.relPath)

			var pdfBuf bytes.Buffer
			if err := asterdoc.ExportPDF(&pdfBuf, sop.wantText, "v1", astercmd.DocumentHash(ld.Source), []string{ld.Source}); err != nil {
				t.Fatalf("%s PDF export failed: %v", sop.name, err)
			}
			if pdfBuf.Len() < 200 || !bytes.HasPrefix(pdfBuf.Bytes(), []byte("%PDF-")) {
				t.Fatalf("%s exported invalid PDF (%d bytes)", sop.name, pdfBuf.Len())
			}
		})
	}

	// Stage 7: Full app restart persistence check (fresh app + shell reading same Vault)
	a.saveVaultWorkspace()
	a2, sh2 := newWiredFSMTestApp(t, vaultRoot)
	st2 := ui.NewTester(func(c *ui.Context) {
		sh2.RenderSplitView(c)
	}, 1360, 880)
	assertBoundedFrame(t, st2, "scenario-after-restart")
	assertGlobalShellInvariants(t, sh2, "scenario-after-restart")
	if len(sh2.VaultFiles) < 4 {
		t.Fatalf("expected >= 4 Vault files after restart, got %d", len(sh2.VaultFiles))
	}
	if active := sh2.ActiveTab(); active == nil || active.Kind != shell.TabFile {
		t.Fatalf("expected active experiment file tab restored after restart, got %+v", active)
	}
	// Confirm persisted external edit survived restart on disk
	if raw, err := os.ReadFile(filepath.Join(vaultRoot, "Projects", "HUVEC-成管.asterml")); err != nil || !strings.Contains(string(raw), "外部同步标记-HUVEC") {
		t.Fatalf("expected HUVEC external edit persisted across restart, err=%v", err)
	}
	_ = a2
}

// TestScenario_ExternalConflictAndInvalidUnitInvariants verifies strict data-safety
// invariants: invalid UCUM units are atomically rejected without mutating disk, and
// stale document hashes return CONFLICT instead of silently overwriting concurrent edits.
func TestScenario_ExternalConflictAndInvalidUnitInvariants(t *testing.T) {
	dir := t.TempDir()
	docPath := filepath.Join(dir, "guard.asterml")
	journalPath := filepath.Join(dir, "journal.json")
	initial := "---\ntitle: 不变量测试\nscheduled_date: \"2026-11-24\"\nstage: planned\n---\n\n#@ 1. 配液与孵育\n"
	if err := asterdoc.WriteFileAtomic(docPath, []byte(initial)); err != nil {
		t.Fatal(err)
	}
	initialHash := astercmd.DocumentHash(initial)

	// 1. Invalid UCUM unit ("倍" / "bogus_unit") on calculator must fail atomically and leave disk untouched.
	badUnitRes := astercmd.ExecuteBatchOnFile(docPath, journalPath, astercmd.BatchRequest{
		ExpectedHash: initialHash,
		Operations: []astercmd.Operation{
			{
				CommandID: "block.insert",
				Args: map[string]any{
					"kind":  "calculator",
					"title": "非法单位稀释",
					"props": map[string]any{
						"mode":         "dilution",
						"stock":        map[string]any{"value": 100, "unit": "倍"},
						"target":       map[string]any{"value": 10, "unit": "ug/mL"},
						"final_volume": map[string]any{"value": 1, "unit": "mL"},
					},
				},
			},
		},
	})
	if badUnitRes.OK {
		t.Fatalf("expected invalid UCUM unit '倍' on calculator to be rejected, but succeeded")
	}
	afterBad, _ := os.ReadFile(docPath)
	if string(afterBad) != initial {
		t.Fatalf("invalid unit write mutated file on disk")
	}

	// 2. Concurrent external edit + stale hash must return CONFLICT without silent overwrite.
	externalEdit := initial + "\n- [ ] 外部协作者新增质控条目\n"
	if err := asterdoc.WriteFileAtomic(docPath, []byte(externalEdit)); err != nil {
		t.Fatal(err)
	}
	conflictRes := astercmd.ExecuteBatchOnFile(docPath, journalPath, astercmd.BatchRequest{
		ExpectedHash: initialHash, // stale!
		Operations: []astercmd.Operation{
			{CommandID: "doc.append_note", Args: map[string]any{"text": "不应覆盖外部修改"}},
		},
	})
	if conflictRes.OK || conflictRes.Error == nil || conflictRes.Error.Code != "CONFLICT" {
		t.Fatalf("expected CONFLICT on concurrent external edit, got %+v", conflictRes)
	}
	afterConflict, _ := os.ReadFile(docPath)
	if string(afterConflict) != externalEdit {
		t.Fatalf("CONFLICT error unexpectedly overwrote external edit")
	}
}

// TestScenario_WarmFrameLatencyP95Budget measures warm interactive frame latency (p50 & p95)
// on the densest real document (96-well RT-qPCR + right drawer) so regressions in input/scroll
// responsiveness are caught quantitatively in CI.
func TestScenario_WarmFrameLatencyP95Budget(t *testing.T) {
	vaultRoot := filepath.Join(t.TempDir(), "PerfVault")
	if err := os.MkdirAll(filepath.Join(vaultRoot, "Lab"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vaultRoot, "Lab", "RT-qPCR.asterml"), []byte(rtQpcrAsterDoc), 0o644); err != nil {
		t.Fatal(err)
	}
	_, sh := newWiredFSMTestApp(t, vaultRoot)
	sh.OpenFileInPane(sh.ActivePaneID, "Lab/RT-qPCR.asterml", "RT-qPCR", shell.ModeEdit, false)
	sh.SetRightDrawer(true, 0)

	st := ui.NewTester(func(c *ui.Context) {
		sh.RenderSplitView(c)
	}, 1440, 900)

	// Warm up 3 frames (font/layout caches)
	for i := 0; i < 3; i++ {
		st.Frame()
	}

	const sampleFrames = 20
	durations := make([]time.Duration, 0, sampleFrames)
	for i := 0; i < sampleFrames; i++ {
		t0 := time.Now()
		st.Frame()
		durations = append(durations, time.Since(t0))
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	p50 := durations[len(durations)/2]
	p95 := durations[(len(durations)*95)/100]
	t.Logf("Warm dense RT-qPCR 96-well frame latency: p50=%v p95=%v max=%v", p50, p95, durations[len(durations)-1])
	// Headless software renderer ceiling: catch severe O(N^2) regressions while tolerating slow CI VMs.
	if p95 > 400*time.Millisecond {
		t.Fatalf("warm p95 frame latency %v exceeded 400ms budget", p95)
	}
}
