package asterdoc

import (
	"bytes"
	"strings"
	"testing"
)

func TestExportPDFDeterministic(t *testing.T) {
	previews := []string{"Plate (plate@1) · p1\n  format: 96"}
	var a, b bytes.Buffer
	if err := ExportPDF(&a, "Doc", "1", "abc", previews); err != nil {
		t.Fatal(err)
	}
	if err := ExportPDF(&b, "Doc", "1", "abc", previews); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("pdf bytes not deterministic")
	}
	if !strings.Contains(string(a.Bytes()), "Revision: 1") {
		t.Fatal("revision missing")
	}
}

func TestFrozenRenderDiffersAfterRevision(t *testing.T) {
	src1 := "```asterlab\nkind: reference\nid: r1\nschema: 1\nref: x\n```\n"
	src2 := "```asterlab\nkind: reference\nid: r1\nschema: 1\nref: y\n```\n"
	d1, _ := Parse(src1)
	d2, _ := Parse(src2)
	if StaticDocumentPreview(d1) == StaticDocumentPreview(d2) {
		t.Fatal("frozen render did not differ after new revision")
	}
}

func TestViewerRejectsWriteWhenReadOnly(t *testing.T) {
	v := &View{ReadOnly: true, Allowed: map[string]bool{}}
	if err := v.Assign([]string{"A1"}, "g"); err == nil {
		t.Fatal("read-only viewer accepted a write")
	}
	// no DB handle field should be exposed
	if _, ok := any(v).(interface{ DB() any }); ok {
		t.Fatal("viewer exposes a DB handle")
	}
}
