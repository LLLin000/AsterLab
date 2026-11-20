package vault

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"asterlab-mygo-prototype/internal/asterdoc"
)

const sampleExperimentA = `---
title: "20261120_CCK8筛选"
scheduled_date: "2026-11-20"
stage: "in_progress"
tags: ["cck8", "drug"]
---

# 20261120_CCK8筛选

` + "```" + `asterlab
id: step-1
kind: step
schema: 1
title: 细胞铺板与加药
blocks:
  - id: blk-prose-1
    kind: prose
    schema: 1
    body: "参考基线方案 [[基线实验]]"
  - id: blk-timer-1
    kind: timer
    schema: 1
    title: 孵育计时
    mode: countdown
    duration:
      value: 60
      unit: min
    status: running
    started_at: "2026-11-20T10:00:00Z"
  - id: blk-grp-1
    kind: group_table
    schema: 1
    rows:
      - id: grp-ctrl
        name: Control
        role: control
        independent_n: 3
      - id: grp-drug
        name: Drug-10uM
        role: treatment
        independent_n: 3
` + "```" + `
`

const sampleExperimentBase = `---
title: "基线实验"
scheduled_date: "2026-11-18"
stage: "completed"
---

# 基线实验

` + "```" + `asterlab
id: step-base
kind: step
schema: 1
title: 基线准备
` + "```" + `
`

func TestVaultIndexSearchBacklinksTimersAndBases(t *testing.T) {
	root := t.TempDir()
	v, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := asterdoc.WriteFileAtomic(v.AbsPath("exp-a.asterml"), []byte(sampleExperimentA)); err != nil {
		t.Fatalf("WriteFileAtomic exp-a: %v", err)
	}
	if err := asterdoc.WriteFileAtomic(v.AbsPath("基线实验.asterml"), []byte(sampleExperimentBase)); err != nil {
		t.Fatalf("WriteFileAtomic base: %v", err)
	}

	idx := NewVaultIndex(v)
	if err := idx.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	hits := idx.SearchFullText("CCK8", 10)
	if len(hits) == 0 || hits[0].RelPath != "exp-a.asterml" {
		t.Fatalf("SearchFullText expected exp-a.asterml hit, got %+v", hits)
	}

	timers := idx.ActiveTimers()
	if len(timers) != 1 || timers[0].TimerID != "blk-timer-1" || timers[0].Status != "running" {
		t.Fatalf("ActiveTimers mismatch: %+v", timers)
	}

	links := idx.BacklinksFor("基线实验.asterml")
	if len(links) != 1 || links[0].SourceRel != "exp-a.asterml" || links[0].Kind != "wikilink" {
		t.Fatalf("BacklinksFor mismatch: %+v", links)
	}

	rows := idx.QueryBases(BasesQuery{Status: "in_progress", Tag: "cck8"})
	if len(rows) != 1 || rows[0].RelPath != "exp-a.asterml" || rows[0].GroupCount != 2 {
		t.Fatalf("QueryBases mismatch: %+v", rows)
	}

	cachePath := v.systemPath(IndexCacheFile)
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatalf("expected index cache at %s: %v", cachePath, err)
	}
	idx2 := NewVaultIndex(v)
	if len(idx2.SearchFullText("基线实验", 5)) == 0 {
		t.Fatalf("expected reloaded index cache to contain base experiment")
	}
}

func TestVaultSnapshotsRenameRewriteAndLocalSplit(t *testing.T) {
	root := t.TempDir()
	v, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := asterdoc.WriteFileAtomic(v.AbsPath("基线实验.asterml"), []byte(sampleExperimentBase)); err != nil {
		t.Fatalf("WriteFileAtomic base: %v", err)
	}
	if err := asterdoc.WriteFileAtomic(v.AbsPath("exp-a.asterml"), []byte(sampleExperimentA)); err != nil {
		t.Fatalf("WriteFileAtomic exp-a: %v", err)
	}

	var gotEvents []VaultEvent
	unsub := v.Events.Subscribe(func(ev VaultEvent) {
		gotEvents = append(gotEvents, ev)
	})
	defer unsub()

	if err := v.RecordSnapshot("exp-a.asterml", sampleExperimentA, true); err != nil {
		t.Fatalf("RecordSnapshot: %v", err)
	}
	snaps := v.ListSnapshots("exp-a.asterml")
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
	}
	restored, err := v.ReadSnapshot("exp-a.asterml", snaps[0].ID)
	if err != nil || !strings.Contains(restored, "20261120_CCK8筛选") {
		t.Fatalf("ReadSnapshot failed: err=%v", err)
	}

	// Rename referenced file -> [[基线实验]] should rewrite to [[新基线实验]]
	newRel, err := v.Rename("基线实验.asterml", "新基线实验.asterml")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if newRel != "新基线实验.asterml" {
		t.Fatalf("unexpected newRel: %s", newRel)
	}
	updatedBytes, err := os.ReadFile(v.AbsPath("exp-a.asterml"))
	if err != nil {
		t.Fatalf("ReadFile exp-a: %v", err)
	}
	if !strings.Contains(string(updatedBytes), "[[新基线实验]]") {
		t.Fatalf("expected wikilink rewrite to [[新基线实验]], got:\n%s", string(updatedBytes))
	}
	if len(gotEvents) == 0 {
		t.Fatalf("expected EventBus notifications on rename/modify")
	}

	// Machine-local fields live in local.json while config.json stays clean
	v.Config.VaultPath = root
	v.Config.LastProjectDir = "Demo"
	if err := v.SaveConfig(); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	cfgBytes, err := os.ReadFile(v.systemPath(ConfigFileName))
	if err != nil {
		t.Fatalf("read config.json: %v", err)
	}
	var rawCfg map[string]any
	if err := json.Unmarshal(cfgBytes, &rawCfg); err != nil {
		t.Fatalf("unmarshal config.json: %v", err)
	}
	if vp, _ := rawCfg["vaultPath"].(string); vp != "" {
		t.Fatalf("expected config.json to omit machine-local vaultPath, got %q", vp)
	}
	localBytes, err := os.ReadFile(v.systemPath(LocalFileName))
	if err != nil {
		t.Fatalf("expected local.json alongside config.json: %v", err)
	}
	if !strings.Contains(string(localBytes), "Demo") {
		t.Fatalf("expected local.json to persist lastProjectDir, got %s", string(localBytes))
	}
}
