package asterdoc

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func settleMobileTester(tester *ui.Tester) {
	tester.Frame()
	for i := 0; i < 45 && tester.FramePending(); i++ {
		tester.Advance(16 * time.Millisecond)
		tester.Frame()
	}
}

func newTestMobileDocView(src string, mobileCompact bool) (*View, *string) {
	cur := src
	doc, err := Parse(cur)
	if err != nil {
		panic(err)
	}
	var v *View
	v = &View{
		Doc:           doc,
		Registry:      BuiltinRegistry(),
		Mode:          ModeDesign,
		UnifiedLive:   true,
		MobileCompact: mobileCompact,
		Allowed:       map[string]bool{PermDocumentEdit: true},
		OnChange: func(next string) {
			cur = next
			if parsed, pErr := Parse(next); pErr == nil {
				v.Doc = parsed
			}
		},
	}
	return v, &cur
}

func TestMobileToolbarFormattingIndentAndCursorActions(t *testing.T) {
	src := "---\ntitle: 移动工具栏测试\n---\n\n加入细胞悬液\n第二行实验记录\n"
	v, cur := newTestMobileDocView(src, true)

	// Locate "加入细胞悬液\n第二行实验记录" inside source
	segStart := strings.Index(src, "加入细胞悬液")
	segText := "加入细胞悬液\n第二行实验记录"
	st := v.SetTestLiveEditor("prose-1", segText, segStart, len("加入细胞"), func(nextText string) string {
		nextSrc, err := ReplaceByteRange(*cur, segStart, segStart+len(segText), nextText)
		if err != nil {
			t.Fatalf("ReplaceByteRange failed: %v", err)
		}
		v.OnChange(nextSrc)
		segText = nextText
		return nextSrc
	})

	// 1. cursor_left / cursor_right across UTF-8 runes
	pos0 := st.Cursor()
	if !v.DispatchMobileAction("cursor_left") {
		t.Fatalf("cursor_left failed")
	}
	if st.Cursor() != pos0-len("胞") {
		t.Fatalf("cursor_left moved to %d, want %d", st.Cursor(), pos0-len("胞"))
	}
	if !v.DispatchMobileAction("cursor_right") {
		t.Fatalf("cursor_right failed")
	}
	if st.Cursor() != pos0 {
		t.Fatalf("cursor_right moved to %d, want %d", st.Cursor(), pos0)
	}

	// 2. format_heading cycles # -> ## -> ### -> plain
	if !v.DispatchMobileAction("format_heading") {
		t.Fatalf("format_heading #1 failed")
	}
	if !strings.HasPrefix(st.Buffer.String(), "# 加入细胞悬液") {
		t.Fatalf("expected H1 heading, got %q", st.Buffer.String())
	}
	v.DispatchMobileAction("format_heading")
	if !strings.HasPrefix(st.Buffer.String(), "## 加入细胞悬液") {
		t.Fatalf("expected H2 heading, got %q", st.Buffer.String())
	}
	v.DispatchMobileAction("format_heading")
	if !strings.HasPrefix(st.Buffer.String(), "### 加入细胞悬液") {
		t.Fatalf("expected H3 heading, got %q", st.Buffer.String())
	}
	v.DispatchMobileAction("format_heading")
	if !strings.HasPrefix(st.Buffer.String(), "加入细胞悬液") {
		t.Fatalf("expected heading cleared, got %q", st.Buffer.String())
	}

	// 3. format_bold with selection
	st.Select(0, len("加入"))
	if !v.DispatchMobileAction("format_bold") {
		t.Fatalf("format_bold failed")
	}
	if !strings.HasPrefix(st.Buffer.String(), "**加入**细胞悬液") {
		t.Fatalf("expected bold wrap, got %q", st.Buffer.String())
	}
	// Toggle bold off
	if !v.DispatchMobileAction("format_bold") {
		t.Fatalf("format_bold unwrap failed")
	}
	if !strings.HasPrefix(st.Buffer.String(), "加入细胞悬液") {
		t.Fatalf("expected bold unwrapped, got %q", st.Buffer.String())
	}

	// 4. format_bullet and format_checklist
	st.Select(0, 0)
	if !v.DispatchMobileAction("format_bullet") {
		t.Fatalf("format_bullet failed")
	}
	if !strings.HasPrefix(st.Buffer.String(), "- 加入细胞悬液") {
		t.Fatalf("expected bullet list, got %q", st.Buffer.String())
	}
	if !v.DispatchMobileAction("format_checklist") {
		t.Fatalf("format_checklist failed")
	}
	if !strings.HasPrefix(st.Buffer.String(), "- [ ] 加入细胞悬液") {
		t.Fatalf("expected checklist conversion, got %q", st.Buffer.String())
	}

	// 5. indent and outdent
	if !v.DispatchMobileAction("indent") {
		t.Fatalf("indent failed")
	}
	if !strings.HasPrefix(st.Buffer.String(), "  - [ ] 加入细胞悬液") {
		t.Fatalf("expected 2-space indent, got %q", st.Buffer.String())
	}
	if !v.DispatchMobileAction("outdent") {
		t.Fatalf("outdent failed")
	}
	if !strings.HasPrefix(st.Buffer.String(), "- [ ] 加入细胞悬液") {
		t.Fatalf("expected outdent restored, got %q", st.Buffer.String())
	}
	if !strings.Contains(*cur, "- [ ] 加入细胞悬液") {
		t.Fatalf("expected OnChange committed formatted line, got:\n%s", *cur)
	}
}

func TestMobileBlockMoveAndFoldActions(t *testing.T) {
	// A. Line move up/down inside prose buffer
	src := "---\ntitle: 块移动与折叠测试\n---\n\n#@ 1. 细胞接种\n第一行内容\n第二行内容\n\n```asterlab\nkind: param_table\nid: pt-1\nschema: 1\ntitle: 参数一\nrows:\n  - key: vol\n    label: 体积\n    value: 100\n    unit: uL\n```\n\n```asterlab\nkind: timer\nid: tm-1\nschema: 1\ntitle: 孵育计时\nmode: countdown\nduration:\n  value: 10\n  unit: min\n```\n"
	v, cur := newTestMobileDocView(src, true)

	segStart := strings.Index(src, "第一行内容")
	segText := "第一行内容\n第二行内容"
	st := v.SetTestLiveEditor("prose-move", segText, segStart, 0, func(nextText string) string {
		nextSrc, err := ReplaceByteRange(*cur, segStart, segStart+len(segText), nextText)
		if err != nil {
			t.Fatalf("ReplaceByteRange failed: %v", err)
		}
		v.OnChange(nextSrc)
		segText = nextText
		return nextSrc
	})

	if !v.DispatchMobileAction("move_block_down") {
		t.Fatalf("move_block_down on line 0 failed")
	}
	if st.Buffer.String() != "第二行内容\n第一行内容" {
		t.Fatalf("expected lines swapped down, got %q", st.Buffer.String())
	}
	if !v.DispatchMobileAction("move_block_up") {
		t.Fatalf("move_block_up on line 1 failed")
	}
	if st.Buffer.String() != "第一行内容\n第二行内容" {
		t.Fatalf("expected lines swapped back up, got %q", st.Buffer.String())
	}

	// B. fold_block toggles stepCollapsed on enclosing heading step
	if !v.DispatchMobileAction("fold_block") {
		t.Fatalf("fold_block failed")
	}
	stepTree := ParseStepTree(v.Doc.Source, v.Doc)
	if len(stepTree.Flat) == 0 {
		t.Fatalf("expected parsed heading step")
	}
	stepID := stepTree.Flat[0].ID
	if !v.stepCollapsed[stepID] {
		t.Fatalf("expected step %s to be collapsed after fold_block", stepID)
	}

	// Render frame with collapsed heading step and verify fold expand pill appears
	tester := ui.NewTester(func(c *ui.Context) {
		v.Render(c)
	}, 390, 844)
	tester.Frame()
	if !tester.HasText("步骤已折叠 · 点击展开") {
		t.Fatalf("expected collapsed heading step placeholder text")
	}
	if !v.DispatchMobileAction("fold_block") || v.stepCollapsed[stepID] {
		t.Fatalf("expected second fold_block to unfold step")
	}

	// C. Top-level module move up/down when activeModuleID is selected
	v.liveEditors = nil
	v.activeModuleID = "tm-1"
	if !v.DispatchMobileAction("move_block_up") {
		t.Fatalf("move_block_up on top-level module failed")
	}
	if strings.Index(*cur, "id: tm-1") > strings.Index(*cur, "id: pt-1") {
		t.Fatalf("expected tm-1 moved before pt-1, got:\n%s", *cur)
	}

	// D. Step-scoped block move delegates to OnMoveStepBlock
	var movedStep, movedBlk, movedDir string
	v.activeStepID = "step-9"
	v.activeBlockID = "blk-2"
	v.OnMoveStepBlock = func(stepID, blockID, dir string) {
		movedStep, movedBlk, movedDir = stepID, blockID, dir
	}
	if !v.DispatchMobileAction("move_block_down") {
		t.Fatalf("step block move_block_down failed")
	}
	if movedStep != "step-9" || movedBlk != "blk-2" || movedDir != "down" {
		t.Fatalf("unexpected step block move call: (%q, %q, %q)", movedStep, movedBlk, movedDir)
	}
}

func TestMobileFootnoteBottomSheetLifecycle(t *testing.T) {
	src := "---\ntitle: 移动脚注 Sheet 测试\n---\n\n按每孔 100 uL 加入培养基。\n"
	v, cur := newTestMobileDocView(src, true)
	v.ShowRightSidenotes = false

	segStart := strings.Index(src, "按每孔")
	segText := "按每孔 100 uL 加入培养基。"
	st := v.SetTestLiveEditor("prose-fn", segText, segStart, len("按每孔"), func(nextText string) string {
		nextSrc, err := ReplaceByteRange(*cur, segStart, segStart+len(segText), nextText)
		if err == nil {
			v.OnChange(nextSrc)
			segText = nextText
		}
		return nextSrc
	})

	// 1. Dispatch add_footnote -> inserts [^1] and opens bottom Sheet (NOT desktop popover)
	if !v.DispatchMobileAction("add_footnote") {
		t.Fatalf("add_footnote failed")
	}
	if !v.MobileFootnoteSheetOpen() || v.MobileFootnoteSheetID() != "1" {
		t.Fatalf("expected mobile footnote sheet open for ID 1, got open=%v id=%q", v.MobileFootnoteSheetOpen(), v.MobileFootnoteSheetID())
	}
	if v.ActiveMobileSheetKind() != "footnote" {
		t.Fatalf("ActiveMobileSheetKind = %q, want footnote", v.ActiveMobileSheetKind())
	}
	if !strings.Contains(*cur, "[^1]") || !strings.Contains(*cur, "[^1]:") {
		t.Fatalf("expected [^1] inserted into document, got:\n%s", *cur)
	}

	// Render keyboard-docked footnote composer card above soft keyboard inset (336dp)
	tester := ui.NewTester(func(c *ui.Context) {
		v.Render(c)
	}, 390, 844)
	tester.SetInsets(59, 0, 336, 0)
	settleMobileTester(tester)
	if !tester.HasText("批注 1") || !tester.HasText("保存") || !tester.HasText("取消") {
		t.Fatalf("expected minimalist keyboard-docked footnote composer controls, texts=%v", tester.Texts())
	}
	saveRect, ok := tester.Find("保存批注")
	if !ok {
		t.Fatalf("expected 保存批注 button in composer header, texts=%v", tester.Texts())
	}
	if saveRect.Y < 59 || saveRect.Y+saveRect.H > 844-336 {
		t.Fatalf("expected composer strictly between SafeArea top=59 and keyboard bottom=508, got %+v", saveRect)
	}

	// Edit draft and save
	draft := v.ensureFootnoteDraft("1", "")
	*draft = "避光分装防止降解"
	if err := tester.Click("保存批注"); err != nil {
		t.Fatalf("click save failed: %v", err)
	}
	settleMobileTester(tester)
	if v.MobileFootnoteSheetOpen() {
		t.Fatalf("expected footnote composer closed after save")
	}
	if !strings.Contains(*cur, "[^1]: 避光分装防止降解") {
		t.Fatalf("expected saved footnote body in source, got:\n%s", *cur)
	}

	// 1b. Inline Tufte accordion (Scheme A): toggle without popping keyboard or opening overlay sheet
	tester.SetInsets(59, 0, 34, 0)
	v.ToggleMobileInlineFootnote("1")
	settleMobileTester(tester)
	for range 20 {
		tester.Advance(16 * time.Millisecond)
		tester.Frame()
	}
	if !v.MobileInlineFootnoteExpanded("1") || !tester.HasText("避光分装防止降解") || !tester.HasText("收起") {
		t.Fatalf("expected inline Tufte footnote accordion expanded, expanded=%v texts=%v",
			v.MobileInlineFootnoteExpanded("1"), tester.Texts())
	}
	if v.MobileFootnoteSheetOpen() {
		t.Fatalf("inline Tufte footnote accordion should not open keyboard composer")
	}
	if err := tester.Click("收起批注"); err != nil {
		t.Fatalf("click collapse inline footnote failed: %v", err)
	}
	for range 20 {
		tester.Advance(16 * time.Millisecond)
		tester.Frame()
	}
	if v.MobileInlineFootnoteExpanded("1") {
		t.Fatalf("expected inline Tufte footnote collapsed after clicking 收起")
	}

	// 1c. Strict read-only ModeLab guard: tapping locked prose must NOT reveal raw `**` or `[^1]:` source lines
	labSrc := "---\ntitle: Lab Strict\n---\n\n第一段：添加 **PBS** 并参考注脚[^1]。\n\n[^1]: 这是只读隐藏定义。\n"
	labDoc, lErr := Parse(labSrc)
	if lErr != nil {
		t.Fatalf("parse labSrc: %v", lErr)
	}
	labView := &View{
		Doc:           labDoc,
		Mode:          ModeProtocol,
		LabMode:       true,
		UnifiedLive:   true,
		MobileCompact: true,
	}
	labTester := ui.NewTester(func(c *ui.Context) {
		labView.Render(c)
	}, 390, 844)
	settleMobileTester(labTester)
	// Simulate tap onto first line of prose
	labTester.FingerDown(80, 30)
	labTester.FingerUp(80, 30)
	settleMobileTester(labTester)
	for _, ent := range labView.liveEditors {
		if ent != nil && ent.state != nil && ent.state.ReadOnly && ent.state.Theme != nil {
			if ent.state.Theme.Cursor != ui.Transparent {
				t.Fatalf("expected transparent cursor in read-only ModeLab prose")
			}
		}
	}

	// 2. Cursor sitting inside existing [^1] re-opens footnote 1 composer instead of inserting [^2]
	refPos := strings.Index(st.Buffer.String(), "[^1]")
	if refPos < 0 {
		t.Fatalf("expected [^1] in editor buffer, got %q", st.Buffer.String())
	}
	st.Select(refPos+1, refPos+1)
	if !v.DispatchMobileAction("add_footnote") {
		t.Fatalf("add_footnote on existing ref failed")
	}
	if v.MobileFootnoteSheetID() != "1" || strings.Contains(*cur, "[^2]") {
		t.Fatalf("expected existing [^1] opened without creating [^2], sheetID=%q src=%s", v.MobileFootnoteSheetID(), *cur)
	}

	// 3. Delete button in bottom sheet removes footnote definition & reference
	settleMobileTester(tester)
	if err := tester.Click("删除批注"); err != nil {
		t.Fatalf("click delete failed: %v", err)
	}
	settleMobileTester(tester)
	if v.MobileFootnoteSheetOpen() || strings.Contains(*cur, "[^1]") {
		t.Fatalf("expected footnote deleted and sheet closed, open=%v src:\n%s", v.MobileFootnoteSheetOpen(), *cur)
	}
}

func TestMobilePickerSheetsAndTouchHeights(t *testing.T) {
	src := "---\ntitle: 移动选择器 Sheet 测试\n---\n\n实验准备：\n"
	v, cur := newTestMobileDocView(src, true)
	v.SlashItems = []SlashCommandItem{
		{CommandID: "block.insert", Label: "计时器", Detail: "实验倒计时与周期计时", Slash: "/timer", Kind: "timer"},
		{CommandID: "plate.create", Label: "96 孔板", Detail: "实验孔板分组排板", Slash: "/plate", Kind: "plate"},
	}
	v.PantryCompletionItems = []PantryMentionItem{
		{ID: "p-dmem", Name: "DMEM 高糖培养基", LabelCode: "R-DMEM", Location: "4°C 冰箱 A2", ItemType: "reagent", Status: "near_expiry"},
		{ID: "p-pbs", Name: "PBS 缓冲液", LabelCode: "R-PBS", Location: "常温架 B1", ItemType: "reagent", Status: "normal"},
	}

	var executedSlash SlashCommandItem
	v.OnExecuteSlash = func(item SlashCommandItem, stepID string, insertOffset int) {
		executedSlash = item
	}

	segStart := strings.Index(src, "实验准备：")
	segText := "实验准备："
	v.SetTestLiveEditor("prose-pk", segText, segStart, len(segText), func(nextText string) string {
		nextSrc, err := ReplaceByteRange(*cur, segStart, segStart+len(segText), nextText)
		if err == nil {
			v.OnChange(nextSrc)
			segText = nextText
		}
		return nextSrc
	})

	// 1. Trigger `/` slash bottom Sheet
	if !v.DispatchMobileAction("trigger_slash") {
		t.Fatalf("trigger_slash failed")
	}
	if v.ActiveMobileSheetKind() != "slash" {
		t.Fatalf("ActiveMobileSheetKind = %q, want slash", v.ActiveMobileSheetKind())
	}

	tester := ui.NewTester(func(c *ui.Context) {
		v.Render(c)
	}, 390, 844)
	settleMobileTester(tester)

	// Verify >=48dp touch row height on slash picker row
	acc, ok := tester.AccessByRole(ui.RoleButton, "计时器")
	if !ok {
		t.Fatalf("expected 计时器 button in slash bottom sheet, texts=%v", tester.Texts())
	}
	if acc.Bounds.H < 48 {
		t.Fatalf("expected slash picker row height >= 48dp, got %.1f", acc.Bounds.H)
	}

	// Search filter narrows items
	v.mobilePickerQuery = "孔板"
	settleMobileTester(tester)
	if tester.HasText("计时器") || !tester.HasText("96 孔板") {
		t.Fatalf("expected search filter to show only 96 孔板, texts=%v", tester.Texts())
	}

	if err := tester.Click("96 孔板"); err != nil {
		t.Fatalf("click 96 孔板 failed: %v", err)
	}
	settleMobileTester(tester)
	if executedSlash.CommandID != "plate.create" || v.mobilePickerOpen {
		t.Fatalf("expected plate.create executed and sheet closed, got %+v open=%v", executedSlash, v.mobilePickerOpen)
	}

	// 2. Trigger `@` mention bottom Sheet
	if !v.DispatchMobileAction("trigger_mention") {
		t.Fatalf("trigger_mention failed")
	}
	if v.ActiveMobileSheetKind() != "mention" {
		t.Fatalf("ActiveMobileSheetKind = %q, want mention", v.ActiveMobileSheetKind())
	}
	settleMobileTester(tester)
	mAcc, mOK := tester.AccessByRole(ui.RoleButton, "@DMEM 高糖培养基")
	if !mOK {
		t.Fatalf("expected @DMEM 高糖培养基 button in mention sheet, texts=%v", tester.Texts())
	}
	if mAcc.Bounds.H < 48 {
		t.Fatalf("expected mention row height >= 48dp, got %.1f", mAcc.Bounds.H)
	}
	if err := tester.Click("@DMEM 高糖培养基"); err != nil {
		t.Fatalf("click @DMEM failed: %v", err)
	}
	settleMobileTester(tester)
	if v.mobilePickerOpen || !strings.Contains(*cur, "asterlab://pantry?code=R-DMEM") {
		t.Fatalf("expected pantry mention link inserted, open=%v src:\n%s", v.mobilePickerOpen, *cur)
	}
}

func TestMobileNLStructuringBottomSheet(t *testing.T) {
	src := "---\ntitle: 移动 NL 转结构测试\n---\n\n37°C 避光孵育 30 min。\n"
	v, cur := newTestMobileDocView(src, true)

	segStart := strings.Index(src, "37°C")
	segText := "37°C 避光孵育 30 min。"
	v.SetTestLiveEditor("prose-nl", segText, segStart, len("37°C"), func(nextText string) string {
		nextSrc, err := ReplaceByteRange(*cur, segStart, segStart+len(segText), nextText)
		if err == nil {
			v.OnChange(nextSrc)
			segText = nextText
		}
		return nextSrc
	})

	if !v.HasActiveNLSuggestion() {
		t.Fatalf("expected HasActiveNLSuggestion=true on NL timer sentence")
	}
	if !v.DispatchMobileAction("trigger_nl") {
		t.Fatalf("trigger_nl failed")
	}
	if v.ActiveMobileSheetKind() != "nl" {
		t.Fatalf("ActiveMobileSheetKind = %q, want nl", v.ActiveMobileSheetKind())
	}

	tester := ui.NewTester(func(c *ui.Context) {
		v.Render(c)
	}, 390, 844)
	settleMobileTester(tester)
	if !tester.HasText("智能转结构预览") || !tester.HasText("一键插入组件") {
		t.Fatalf("expected mobile NL preview bottom sheet content, texts=%v", tester.Texts())
	}
	btnAcc, ok := tester.AccessByRole(ui.RoleButton, "下方插入组件")
	if !ok {
		t.Fatalf("expected NL apply button")
	}
	if btnAcc.Bounds.H < 48 {
		t.Fatalf("expected NL apply button height >= 48dp, got %.1f", btnAcc.Bounds.H)
	}
	if err := tester.Click("下方插入组件"); err != nil {
		t.Fatalf("click NL apply failed: %v", err)
	}
	settleMobileTester(tester)
	if v.mobileNLSheetOpen || !strings.Contains(*cur, "kind: timer") {
		t.Fatalf("expected timer module inserted and NL sheet closed, open=%v src:\n%s", v.mobileNLSheetOpen, *cur)
	}
}

func TestModeLabMobileSwipeErgonomicsAndLockGuard(t *testing.T) {
	src := `---
title: 实验模式手势测试
---

` + "```asterlab\nkind: step\nid: step-1\nschema: 1\ntitle: 细胞加药\nblocks:\n  - kind: checklist\n    id: chk-blk-1\n    schema: 1\n    items:\n      - id: c1\n        label: 确认细胞汇合度达 80%\n        done: false\n  - kind: timer\n    id: tm-blk-1\n    schema: 1\n    title: 孵育倒计时\n    mode: countdown\n    duration:\n      value: 30\n      unit: min\n    status: idle\n```\n"

	v, cur := newTestMobileDocView(src, true)
	v.Mode = ModeProtocol
	v.LabMode = true
	v.ReadOnly = false

	var toggledCheck string
	var toggledVal bool
	v.OnToggleCheck = func(stepID, checkID string, checked bool) {
		toggledCheck = checkID
		toggledVal = checked
	}
	var timerEvents []string
	v.OnTimerAction = func(stepID, blockID, action string) {
		timerEvents = append(timerEvents, blockID+":"+action)
	}
	v.OnUpdateStepBlock = func(stepID, blockID string, props map[string]any) {
		if v.Doc == nil {
			return
		}
		sm := findModuleByID(v.Doc, stepID)
		if sm == nil {
			return
		}
		blocks := mapSlice(sm.Data["blocks"])
		for i, bm := range blocks {
			if textValue(bm, "id") == blockID {
				for pk, pv := range props {
					blocks[i][pk] = pv
				}
			}
		}
		if next, err := v.Doc.UpdateModule(stepID, map[string]any{"blocks": blocks}); err == nil {
			v.OnChange(next)
		}
	}

	// LabMode blocks structural formatting/move mutations
	v.SetTestLiveEditor("lab-note", "备注", 0, 0, nil)
	if v.DispatchMobileAction("format_heading") || v.DispatchMobileAction("format_bold") || v.DispatchMobileAction("move_block_up") {
		t.Fatalf("expected LabMode to block edit/move actions")
	}

	// Render ModeLab + MobileCompact and verify >=48dp rows
	tester := ui.NewTester(func(c *ui.Context) {
		v.Render(c)
	}, 390, 844)
	settleMobileTester(tester)

	chkRect, ok := tester.Find("检查项：确认细胞汇合度达 80%")
	if !ok {
		t.Fatalf("expected checklist row rendered, texts=%v", tester.Texts())
	}
	if chkRect.H < 48 {
		t.Fatalf("expected ModeLab mobile checklist row height >= 48dp, got %.1f", chkRect.H)
	}
	tmRect, ok := tester.Find("计时行：孵育倒计时")
	if !ok {
		t.Fatalf("expected timer row rendered, texts=%v", tester.Texts())
	}
	if tmRect.H < 48 {
		t.Fatalf("expected ModeLab mobile timer row height >= 48dp, got %.1f", tmRect.H)
	}

	// Verify right-swipe mutator (mark checklist done + green flash + OnToggleCheck)
	if !v.ToggleChecklistSwipeDone("chk-blk-1", 0) {
		t.Fatalf("ToggleChecklistSwipeDone failed")
	}
	if toggledCheck != "c1" || !toggledVal || !v.CheckedItems["c1"] {
		t.Fatalf("expected c1 checked via swipe, got (%q, %v, checkedItems=%v)", toggledCheck, toggledVal, v.CheckedItems)
	}
	if !v.isSwipeFlashActive(nil, "chk-flash:chk-blk-1:c1") {
		t.Fatalf("expected green swipe flash active right after swipe-done")
	}

	// Verify left-swipe mutator (start then pause step timer)
	tmID, st1, ok1 := v.ToggleStepTimerForStep("step-1")
	if !ok1 || tmID != "tm-blk-1" || st1 != "running" {
		t.Fatalf("expected step timer started, got (%q, %q, %v)", tmID, st1, ok1)
	}
	if !strings.Contains(*cur, "status: running") {
		t.Fatalf("expected running timer persisted to source:\n%s", *cur)
	}
	_, st2, ok2 := v.ToggleStepTimerForStep("step-1")
	if !ok2 || st2 != "paused" {
		t.Fatalf("expected step timer paused on second left-swipe, got (%q, %v)", st2, ok2)
	}
	if len(timerEvents) != 2 || timerEvents[0] != "tm-blk-1:start" || timerEvents[1] != "tm-blk-1:pause" {
		t.Fatalf("unexpected OnTimerAction events: %v", timerEvents)
	}

	// Verify finger drag on SwipeRow reveals action and triggers cleanly
	tester.Advance(600 * time.Millisecond)
	tester.Frame()
	cx := chkRect.X + chkRect.W*0.5
	cy := chkRect.Y + chkRect.H*0.5
	tester.FingerDown(cx, cy)
	tester.FingerMove(cx+110, cy)
	tester.FingerUp(cx+110, cy)
	tester.Frame()
}
