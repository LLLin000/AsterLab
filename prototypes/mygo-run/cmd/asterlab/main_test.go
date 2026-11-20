package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/vault"
)

const cliSampleValidCCK8 = `---
title: "CLI CCK8 Test"
scheduled_date: "2026-11-20"
stage: "in_progress"
tags: ["cck8"]
---

` + "```" + `asterlab
id: step-1
kind: step
schema: 1
title: 铺板与读数
blocks:
  - id: blk-grp
    kind: group_table
    schema: 1
    rows:
      - id: g-ctrl
        name: Control
        role: control
        independent_n: 3
      - id: g-drug
        name: Drug
        role: treatment
        independent_n: 3
  - id: blk-dt
    kind: data_table
    schema: 1
    title: OD450
    columns:
      - key: well
        label: 孔位
        value_kind: text
      - key: group
        label: 分组
        value_kind: text
      - key: value
        label: OD450
        value_kind: number
    rows:
      - well: A1
        group: Control
        value: 1.0
      - well: B1
        group: Drug
        value: 0.5
` + "```" + `
`

const cliSampleBlocking = `---
title: "Blocked Calc Test"
scheduled_date: "2026-11-20"
---

` + "```" + `asterlab
id: step-1
kind: step
schema: 1
title: 浓度越界
blocks:
  - id: blk-calc
    kind: calculator
    schema: 1
    mode: formula
    formula: "c2 / c1"
    rows:
      - key: c1
        value: 10
      - key: c2
        value: 20
` + "```" + `
`

const cliSampleInvalidSyntax = `---
title: "Broken Syntax Test"
scheduled_date: "2026-11-20"
---

` + "```" + `asterlab
id: step-1
kind: step
schema: 1
title: 母液越界
blocks:
  - id: blk-calc
    kind: calculator
    schema: 1
    mode: dilution
    stock:
      value: 10
      unit: mM
    target:
      value: 20
      unit: mM
    final_volume:
      value: 5
      unit: mL
` + "```" + `
`

func TestAsterLabVaultCLI(t *testing.T) {
	root := t.TempDir()
	v, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open: %v", err)
	}
	if err := asterdoc.WriteFileAtomic(v.AbsPath("cck8.asterml"), []byte(cliSampleValidCCK8)); err != nil {
		t.Fatal(err)
	}
	if err := asterdoc.WriteFileAtomic(v.AbsPath("blocked.asterml"), []byte(cliSampleBlocking)); err != nil {
		t.Fatal(err)
	}
	if err := asterdoc.WriteFileAtomic(v.AbsPath("invalid.asterml"), []byte(cliSampleInvalidSyntax)); err != nil {
		t.Fatal(err)
	}

	run := func(args ...string) (int, map[string]any) {
		var out, errBuf bytes.Buffer
		full := append([]string{"--vault", root}, args...)
		code := runCLI(full, &out, &errBuf)
		var parsed map[string]any
		if uErr := json.Unmarshal(out.Bytes(), &parsed); uErr != nil {
			t.Fatalf("args=%v invalid JSON output (%v): %q", args, uErr, out.String())
		}
		return code, parsed
	}

	if code, res := run("tools", "--json"); code != exitOK || res["ok"] != true {
		t.Fatalf("tools failed: code=%d res=%+v", code, res)
	}

	if code, res := run("vault.list", "--status", "in_progress", "--tag", "cck8"); code != exitOK || res["count"] != float64(1) {
		t.Fatalf("vault.list failed: code=%d res=%+v", code, res)
	}

	if code, res := run("vault.search", "OD450"); code != exitOK || res["count"] != float64(1) {
		t.Fatalf("vault.search failed: code=%d res=%+v", code, res)
	}

	if code, res := run("asterdoc.read", "cck8.asterml"); code != exitOK || res["ok"] != true {
		t.Fatalf("asterdoc.read failed: code=%d res=%+v", code, res)
	}

	if code, res := run("asterdoc.assess", "cck8.asterml"); code != exitOK || res["ok"] != true {
		t.Fatalf("asterdoc.assess valid failed: code=%d res=%+v", code, res)
	}

	if code, res := run("asterdoc.assess", "blocked.asterml"); code != exitBlocked || res["ok"] != false {
		t.Fatalf("expected exitBlocked (%d) for TARGET_EXCEEDS_STOCK, got code=%d res=%+v", exitBlocked, code, res)
	}

	if code, _ := run("asterdoc.assess", "invalid.asterml"); code != exitInvalid {
		t.Fatalf("expected exitInvalid (%d), got %d", exitInvalid, code)
	}

	if code, res := run("asterdoc.analyze", "cck8.asterml"); code != exitOK || res["count"] != float64(1) {
		t.Fatalf("asterdoc.analyze failed: code=%d res=%+v", code, res)
	}

	if code, _ := run("asterdoc.read", "missing.asterml"); code != exitNotFound {
		t.Fatalf("expected exitNotFound (%d), got %d", exitNotFound, code)
	}
}
