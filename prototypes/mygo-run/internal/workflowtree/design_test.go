package workflowtree

import (
	"strings"
	"testing"

	"asterlab-mygo-prototype/internal/asterdoc"
)

type staticBlocks map[string]BlockDefinition

func (s staticBlocks) BlockDefinition(versionID string) (BlockDefinition, error) {
	return s[versionID], nil
}

func rnaBlock(versionID string) BlockDefinition {
	steps := make([]BlockStep, 0, 10)
	titles := []string{"收集细胞", "裂解与匀浆", "加乙醇", "上柱结合", "Wash I", "Wash II 第一次", "Wash II 第二次", "干柱离心", "RNA 洗脱", "RNA 质检与保存"}
	for i, title := range titles {
		steps = append(steps, BlockStep{
			UseID:                title,
			Path:                 "root/" + pad2(i+1) + "-op",
			Title:                title,
			Instruction:          title + " instruction",
			SourceBlockVersionID: versionID,
			Parameters: []BlockParameter{{
				Key: "volume", Label: "体积", Value: "500", Unit: "µL",
			}},
		})
	}
	return BlockDefinition{BlockVersionID: versionID, Ready: true, Steps: steps}
}

func pad2(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

func TestBuildDesignWithoutRun(t *testing.T) {
	provider := staticBlocks{"c23df8e1-3f2a-4449-8ac9-a4bb7256a113": rnaBlock("c23df8e1-3f2a-4449-8ac9-a4bb7256a113")}
	tree, err := BuildDesign(parentDoc(t), provider)
	if err != nil {
		t.Fatal(err)
	}
	if !tree.Design {
		t.Fatal("design tree not marked")
	}
	if err := tree.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(tree.Roots) != 4 {
		t.Fatalf("roots = %d, want 4", len(tree.Roots))
	}
	rna := tree.Roots[0]
	if rna.Identity != IdentityUse || !rna.Composite || rna.Total != 10 || len(rna.Children) != 10 {
		t.Fatalf("RNA design node = %+v", rna)
	}
	if rna.BlockVersionID != "c23df8e1-3f2a-4449-8ac9-a4bb7256a113" {
		t.Fatalf("RNA block = %q", rna.BlockVersionID)
	}
	for _, child := range rna.Children {
		if child.Identity != IdentityDefinition || child.Status != "" || child.BlockVersionID == "" {
			t.Fatalf("definition child = %+v", child)
		}
		if child.Instruction == "" || len(child.Parameters) != 1 || child.Parameters[0].Label != "体积" {
			t.Fatalf("definition detail = %+v", child)
		}
	}
	if tree.Roots[1].Composite {
		t.Fatalf("leaf step unexpectedly composite: %+v", tree.Roots[1])
	}
}

func TestBuildDesignRejectsMissingBlockProvider(t *testing.T) {
	tree, err := BuildDesign(parentDoc(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Validate() == nil || !strings.Contains(tree.Validate().Error(), "provider is unavailable") {
		t.Fatalf("Validate() = %v", tree.Validate())
	}
	if tree.Roots[0].Composite || len(tree.Roots[0].Children) != 0 {
		t.Fatalf("missing provider guessed children: %+v", tree.Roots[0])
	}
}

func TestBuildDesignReusesSameBlockVersionIndependently(t *testing.T) {
	source := parentSourceText(t)
	blockStart := strings.Index(source, "kind: step\nid: step-rna-extract")
	if blockStart < 0 {
		t.Fatal("RNA step not found")
	}
	blockEnd := strings.Index(source[blockStart:], "```")
	if blockEnd < 0 {
		t.Fatal("RNA step fence not found")
	}
	block := source[blockStart : blockStart+blockEnd]
	copyBlock := strings.NewReplacer(
		"step-rna-extract", "step-rna-extract-copy",
		"root/rna-extraction", "root/rna-extraction-copy",
		"order: 1", "order: 5",
		"01 RNA 提取", "01 RNA 提取（复用）",
	).Replace(block)
	insertAt := blockStart + blockEnd + 3
	source = source[:insertAt] + "\n\n```asterlab\n" + copyBlock + "```" + source[insertAt:]
	provider := staticBlocks{"c23df8e1-3f2a-4449-8ac9-a4bb7256a113": rnaBlock("c23df8e1-3f2a-4449-8ac9-a4bb7256a113")}
	tree, err := BuildDesign(&asterdoc.FrozenAsterDoc{DesignID: "d", Revision: 1, Source: source}, provider)
	if err != nil {
		t.Fatal(err)
	}
	if err := tree.Validate(); err != nil {
		t.Fatal(err)
	}
	var uses []*Node
	for _, root := range tree.Roots {
		if root.Composite {
			uses = append(uses, root)
		}
	}
	if len(uses) != 2 {
		t.Fatalf("composite uses = %d, want 2", len(uses))
	}
	if uses[0].ID == uses[1].ID || uses[0].Path == uses[1].Path {
		t.Fatalf("uses merged: %+v %+v", uses[0], uses[1])
	}
	if uses[0].Children[0].Path == uses[1].Children[0].Path {
		t.Fatalf("definition children merged: %q", uses[0].Children[0].Path)
	}
	for _, use := range uses {
		if use.Total != 10 || len(use.Children) != 10 {
			t.Fatalf("use %s total=%d children=%d", use.Path, use.Total, len(use.Children))
		}
	}
}

func TestBuildDesignKeepsOldVersionReference(t *testing.T) {
	oldVersion := "c23df8e1-3f2a-4449-8ac9-a4bb7256a113"
	newVersion := "new-version"
	provider := staticBlocks{
		oldVersion: rnaBlock(oldVersion),
		newVersion: rnaBlock(newVersion),
	}
	source := parentSourceText(t)
	// Add a second reference and a second use while leaving the old step and
	// its reference untouched.
	source = strings.Replace(source, "view: protocol_block\n```", "view: protocol_block\n```\n\n```asterlab\nkind: reference\nid: ref-rna-block-v2\nschema: 1\nref: "+newVersion+"\nview: protocol_block\n```", 1)
	blockStart := strings.Index(source, "kind: step\nid: step-rna-extract")
	blockEnd := strings.Index(source[blockStart:], "```")
	block := source[blockStart : blockStart+blockEnd]
	copyBlock := strings.NewReplacer(
		"step-rna-extract", "step-rna-extract-v2",
		"root/rna-extraction", "root/rna-extraction-v2",
		"order: 1", "order: 6",
		"ref: ref-rna-block", "ref: ref-rna-block-v2",
		"01 RNA 提取", "01 RNA 提取（新版本）",
	).Replace(block)
	insertAt := blockStart + blockEnd + 3
	source = source[:insertAt] + "\n\n```asterlab\n" + copyBlock + "```" + source[insertAt:]
	tree, err := BuildDesign(&asterdoc.FrozenAsterDoc{DesignID: "d", Revision: 1, Source: source}, provider)
	if err != nil {
		t.Fatal(err)
	}
	if err := tree.Validate(); err != nil {
		t.Fatal(err)
	}
	byPath := map[string]*Node{}
	for _, root := range tree.Roots {
		byPath[root.Path] = root
	}
	oldUse := byPath["root/rna-extraction"]
	newUse := byPath["root/rna-extraction-v2"]
	if oldUse == nil || newUse == nil {
		t.Fatalf("uses missing: %+v", byPath)
	}
	if oldUse.BlockVersionID != oldVersion || newUse.BlockVersionID != newVersion {
		t.Fatalf("versions = %q %q", oldUse.BlockVersionID, newUse.BlockVersionID)
	}
	if oldUse.Children[0].BlockVersionID != oldVersion || newUse.Children[0].BlockVersionID != newVersion {
		t.Fatalf("child versions = %q %q", oldUse.Children[0].BlockVersionID, newUse.Children[0].BlockVersionID)
	}
}
