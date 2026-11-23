package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIAgentCommandFullLoop(t *testing.T) {
	vaultDir := t.TempDir()
	expPath := filepath.Join(vaultDir, "huvec.asterml")
	initial := "---\ntitle: HUVEC 成管\n---\n\n自由记录：准备测试 VEGF 促成管效果。\n"
	if err := os.WriteFile(expPath, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	run := func(stdin string, args ...string) (int, map[string]any) {
		var out, errBuf bytes.Buffer
		full := append([]string{"--vault", vaultDir}, args...)
		code := runCLIWithStdin(full, strings.NewReader(stdin), &out, &errBuf)
		var parsed map[string]any
		if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
			t.Fatalf("CLI %v invalid JSON (%q): %v (stderr=%s)", args, out.String(), err, errBuf.String())
		}
		return code, parsed
	}

	// 1. Discover commands
	code, listRes := run("", "cmd.list")
	if code != exitOK || listRes["ok"] != true {
		t.Fatalf("cmd.list failed: code=%d res=%v", code, listRes)
	}

	// 2. Read initial objects & docHash
	code, objRes := run("", "asterdoc.objects", "huvec.asterml")
	if code != exitOK {
		t.Fatalf("asterdoc.objects failed: %v", objRes)
	}
	docHash, _ := objRes["docHash"].(string)
	if !strings.HasPrefix(docHash, "sha256:") {
		t.Fatalf("expected sha256 docHash, got %q", docHash)
	}

	// 3. Preview batch (must not mutate file on disk)
	batch := `{
		"expected_hash": "` + docHash + `",
		"idempotency_key": "agent-slice-1",
		"operations": [
			{"command_id": "group.create", "client_ref": "g_vegf", "args": {"name": "VEGF 组", "role": "treatment"}},
			{"command_id": "plate.create", "client_ref": "p12", "args": {"title": "12孔板", "format": 12}},
			{"command_id": "plate.assign_wells", "args": {"plate_id": "p12", "group_id": "g_vegf", "wells": ["A1", "A2"]}},
			{"command_id": "asset.embed", "args": {"asset": "./assets/a1_6h.png", "targets": ["@p12:A1"], "timepoint": "6h", "fov": 1, "role": "obs"}}
		]
	}`
	code, prevRes := run(batch, "asterdoc.preview", "huvec.asterml")
	if code != exitOK || prevRes["ok"] != true || prevRes["preview_only"] != true {
		t.Fatalf("asterdoc.preview failed: code=%d res=%v", code, prevRes)
	}
	afterPreview, _ := os.ReadFile(expPath)
	if string(afterPreview) != initial {
		t.Fatalf("preview mutated file on disk")
	}

	// 4. Execute batch atomically
	code, execRes := run(batch, "asterdoc.exec", "huvec.asterml")
	if code != exitOK || execRes["ok"] != true {
		t.Fatalf("asterdoc.exec failed: code=%d res=%v", code, execRes)
	}

	// 5. Retry with same idempotency_key -> replayed without duplicating objects
	code, retryRes := run(batch, "asterdoc.exec", "huvec.asterml")
	if code != exitOK || retryRes["replayed"] != true {
		t.Fatalf("expected idempotent replay, got code=%d res=%v", code, retryRes)
	}

	// 6. Query resolved relations (must resolve plate + group context cleanly)
	code, relRes := run("", "asterdoc.relations", "huvec.asterml")
	if code != exitOK || relRes["ok"] != true {
		t.Fatalf("asterdoc.relations failed: code=%d res=%v", code, relRes)
	}
	if int(relRes["count"].(float64)) != 1 {
		t.Fatalf("expected 1 relation, got %v", relRes)
	}

	// 7. Validate document
	code, valRes := run("", "asterdoc.validate", "huvec.asterml")
	if code != exitOK || valRes["ok"] != true {
		t.Fatalf("asterdoc.validate failed: code=%d res=%v", code, valRes)
	}
}

func TestCLIAsterdocSuggestFileAndTextMode(t *testing.T) {
	vaultDir := t.TempDir()
	expPath := filepath.Join(vaultDir, "cfu.asterml")
	docSrc := strings.Join([]string{
		"---",
		"title: CFU 计划",
		"scheduled_date: \"2026-11-24\"",
		"stage: planned",
		"---",
		"",
		"#@ 1. 恒温培养与稀释",
		"- [ ] 将母液从 100 ug/mL 稀释至 50 ng/mL，配制 8 mL 工作液",
		"- [ ] 37°C 恒温倒置培养 24 h",
		"- [ ] 无菌 PBS 吹打混匀",
		"",
	}, "\n")
	if err := os.WriteFile(expPath, []byte(docSrc), 0o644); err != nil {
		t.Fatal(err)
	}

	run := func(args ...string) (int, map[string]any) {
		var out, errBuf bytes.Buffer
		full := append([]string{"--vault", vaultDir}, args...)
		code := runCLIWithStdin(full, strings.NewReader(""), &out, &errBuf)
		var parsed map[string]any
		if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
			t.Fatalf("CLI %v invalid JSON (%q): %v (stderr=%s)", args, out.String(), err, errBuf.String())
		}
		return code, parsed
	}

	// 1. File mode
	code, res := run("asterdoc.suggest", "cfu.asterml")
	if code != exitOK || res["ok"] != true {
		t.Fatalf("asterdoc.suggest file mode failed: code=%d res=%v", code, res)
	}
	if int(res["count"].(float64)) < 2 {
		t.Fatalf("expected >= 2 suggestions for cfu.asterml, got %v", res)
	}

	// 2. Single-sentence --text mode
	code, textRes := run("asterdoc.suggest", "--text", "37°C 孵育 30 min")
	if code != exitOK || textRes["ok"] != true {
		t.Fatalf("asterdoc.suggest --text mode failed: code=%d res=%v", code, textRes)
	}
	if int(textRes["count"].(float64)) != 1 {
		t.Fatalf("expected 1 suggestion for single timer sentence, got %v", textRes)
	}

	// 3. UTF-8 BOM piped via PowerShell stdin into asterdoc.preview must parse cleanly
	var bomOut, bomErr bytes.Buffer
	bomPayload := "\xef\xbb\xbf" + `{"operations":[{"command_id":"doc.append_note","args":{"text":"BOM-OK"}}]}`
	bomCode := runCLIWithStdin([]string{"--vault", vaultDir, "asterdoc.preview", "cfu.asterml"}, strings.NewReader(bomPayload), &bomOut, &bomErr)
	if bomCode != exitOK {
		t.Fatalf("asterdoc.preview with UTF-8 BOM failed (code=%d): %s %s", bomCode, bomOut.String(), bomErr.String())
	}

	// 4. Absolute path inside --vault must resolve cleanly without double-joining paths
	code, absRes := run("asterdoc.validate", expPath)
	if code != exitOK || absRes["ok"] != true {
		t.Fatalf("asterdoc.validate with absolute path inside --vault failed: code=%d res=%v", code, absRes)
	}
}
