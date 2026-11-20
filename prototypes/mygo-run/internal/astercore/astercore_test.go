package astercore

import (
	"testing"

	"asterlab-mygo-prototype/internal/asterdoc"
)

func TestAssessDocumentGradedDiagnostics(t *testing.T) {
	badSrc := `---
title: ""
status: "draft"
---

` + "```" + `asterlab
id: step-1
kind: step
schema: 1
title: 稀释与分组
blocks:
  - id: blk-grp
    kind: group_table
    schema: 1
    rows:
      - id: grp-treated
        name: Treated
        role: treatment
        independent_n: 0
  - id: blk-calc
    kind: calculator
    schema: 1
    mode: dilution
    stock:
      value: 10
      unit: mM
    target:
      value: 25
      unit: mM
    final_volume:
      value: 5
      unit: mL
    rows:
      - key: c1
        value: 10
      - key: c2
        value: 25
` + "```" + `
`
	doc, err := asterdoc.Parse(badSrc)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	rep := AssessDocument(doc, nil)
	if rep.Ready {
		t.Fatalf("expected Ready=false when blocking issues exist")
	}
	if rep.Blocking == 0 || rep.Confirm == 0 {
		t.Fatalf("expected blocking + needs_confirmation diagnostics, got %+v", rep)
	}
	hasCode := func(code string) bool {
		for _, d := range rep.Diagnostics {
			if d.Code == code {
				return true
			}
		}
		return false
	}
	for _, want := range []string{"TARGET_EXCEEDS_STOCK", "ARRIVE_MISSING_N", "NO_CONTROL_GROUP", "UNSCHEDULED_DATE"} {
		if !hasCode(want) {
			t.Fatalf("expected diagnostic %s, got %+v", want, rep.Diagnostics)
		}
	}
}

func TestAnalyzeDocumentCCK8AndQPCR(t *testing.T) {
	cck8Src := `---
title: "CCK-8 细胞活力"
date: "2026-11-20"
---

` + "```" + `asterlab
id: step-1
kind: step
schema: 1
title: 分组与读板
blocks:
  - id: blk-grp
    kind: group_table
    schema: 1
    rows:
      - id: grp-ctrl
        name: Control
        role: control
        independent_n: 3
      - id: grp-treated
        name: Treated
        role: treatment
        independent_n: 3
  - id: blk-dt
    kind: data_table
    schema: 1
    title: "OD450"
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
        value: 1.00
      - well: A2
        group: Control
        value: 1.00
      - well: B1
        group: Treated
        value: 0.50
      - well: B2
        group: Treated
        value: 0.50
` + "```" + `
`
	doc, err := asterdoc.Parse(cck8Src)
	if err != nil {
		t.Fatalf("Parse CCK8: %v", err)
	}
	sums := AnalyzeDocument(doc, "cck8-doc")
	if len(sums) != 1 || sums[0].Kind != "cck8" || sums[0].CCK8 == nil {
		t.Fatalf("expected 1 cck8 summary, got %+v", sums)
	}

	qpcrSrc := `---
title: "RT-qPCR 相对定量"
date: "2026-11-20"
---

` + "```" + `asterlab
id: step-1
kind: step
schema: 1
title: Ct 读数
blocks:
  - id: blk-grp
    kind: group_table
    schema: 1
    rows:
      - id: grp-ctrl
        name: Ctrl
        role: control
        independent_n: 3
      - id: grp-drug
        name: Drug
        role: treatment
        independent_n: 3
  - id: blk-dt
    kind: data_table
    schema: 1
    title: "qPCR Cq"
    columns:
      - key: sample
        label: 样本
        value_kind: text
      - key: group
        label: 分组
        value_kind: text
      - key: gene
        label: 基因
        value_kind: text
      - key: value
        label: Cq
        value_kind: number
    rows:
      - sample: Ctrl
        group: Ctrl
        gene: GAPDH
        value: 18.0
      - sample: Ctrl
        group: Ctrl
        gene: TP53
        value: 22.0
      - sample: Drug
        group: Drug
        gene: GAPDH
        value: 18.0
      - sample: Drug
        group: Drug
        gene: TP53
        value: 20.0
` + "```" + `
`
	qDoc, err := asterdoc.Parse(qpcrSrc)
	if err != nil {
		t.Fatalf("Parse qPCR: %v", err)
	}
	qSums := AnalyzeDocument(qDoc, "qpcr-doc")
	if len(qSums) != 1 || qSums[0].Kind != "qpcr" || qSums[0].QPCR == nil {
		t.Fatalf("expected 1 qpcr summary, got %+v", qSums)
	}
}
