package main

// Design surface: workflow canvas, document editor and Run creation. Owned by
// the Design Owner after Wave 1 extraction; routing lives in main.go.

import (
	"fmt"

	"asterlab-mygo-prototype/internal/agentedit"
	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/designer"
	"asterlab-mygo-prototype/internal/theme"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
)
var agentEditStates = map[*app]*agentedit.State{}
func proposalState(a *app) *agentedit.State {
	if s:=agentEditStates[a]; s!=nil{return s}
	s:=&agentedit.State{Client:agentedit.CLIClient{},ConfirmedBy:"researcher",OnCommitted:func(agentedit.CommitResult){if a.designDoc!=nil{a.loadDesign(a.designDoc.DesignID)}}}
	agentEditStates[a]=s;return s
}
// Agent proposal entry stays secondary to Create Run — UX-P1/P2/P3/P5 and UX-LAYOUT.
func renderAgentProposal(c *ui.Context,a *app) {
	s:=proposalState(a)
	if components.Button(c,"Agent 提案",components.ButtonSecondary).Clicked(){s.Open=true;s.Closed=false}
	if s.Open {components.Dialog(c,&s.Open,components.DialogOptions{Label:"Agent 提案"},func(panel ui.Element){panel.FillWidth().MaxWidth(720).MaxWidthPercent(94).MaxHeightPercent(90).Children(func(){agentedit.View(c,s)})});if !s.Open{s.Closed=true}}
}

func designPage(c *ui.Context, a *app) {
	if a.designDoc != nil {
		asterDesignPage(c, a)
		return
	}
	if a.designDocErr != "" {
		ui.Column(c).Fill().Center().Gap(c.Theme().Space(2)).Children(func() {
			ui.Text(c, "AsterDoc 设计无法读取").FontSize(16).Bold().TextColor(c.Theme().Danger)
			ui.Text(c, a.designDocErr).FontSize(13).TextColor(c.Theme().TextMuted)
			renderAgentProposal(c,a)
		})
		return
	}
	if a.designer != nil {
		designer.View(c, a.designer)
		renderAgentProposal(c,a)
		return
	}
	t := c.Theme()
	message := "请从项目页选择一个已保存的设计。"
	if a.designSvc == nil { message = "设计服务不可用。" }
	ui.Column(c).Fill().Center().Gap(t.Space(2)).Children(func() {
		components.Text(c, "还没有打开的设计").FontSize(t.Rem(1.15)).FontWeight(600)
		components.Text(c, message).TextColor(t.TextMuted)
		renderAgentProposal(c,a)
	})
}

// asterDesignPage is the document-backed Design surface. Professional editors
// issue semantic commands; this slice supports plate editing and Run creation.
func asterDesignPage(c *ui.Context, a *app) {
	t := c.Theme()
	c.OnShortcut(ui.Cmd, ui.KeyZ, func() { a.designUndo() })
	c.OnShortcut(ui.Cmd|ui.Shift, ui.KeyZ, func() { a.designRedo() })
	c.OnShortcut(ui.Cmd, ui.KeyY, func() { a.designRedo() })
	doc, err := asterdoc.Parse(a.designDoc.Source)
	if err != nil {
		ui.Text(c, "AsterDoc 语法错误："+err.Error()).Padding(t.Space(6)).TextColor(t.Danger)
		return
	}
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).FillWidth().Wrap().Padding(t.Space(2), t.Space(4)).Border(1, t.Border).
			Background(t.Surface).AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
			ui.Text(c, "AsterDoc 设计").FontSize(16).Bold().TextColor(t.Text)
			ui.Text(c, fmt.Sprintf("rev %d", a.designDoc.Revision)).FontSize(11).
				TextColor(t.TextMuted)
			if a.readiness != nil {
				label, color := "草稿", t.TextMuted
				switch a.readiness.State {
				case "incomplete":
					label = "待完善"
					color = t.Warning
				case "warning":
					label = "可创建（有警告）"
					color = t.Warning
				case "ready":
					label = "可创建运行"
					color = t.Success
				}
				ui.Text(c, label).FontSize(10).Bold().TextColor(color)
			}
			tab := a.designTab
			components.Tabs(c, &tab, components.TabsSegmented, "流程", "文档")
			a.designTab = tab
			ui.Spacer(c)
			undo := components.IconButton(c, lucide.Undo2(), "撤销", components.IconButtonCompact)
			undo.Disabled(len(a.designHistory) == 0 && len(a.asterDraftUndo) == 0)
			if undo.Clicked() {
				a.designUndo()
			}
			redo := components.IconButton(c, lucide.Redo2(), "重做", components.IconButtonCompact)
			redo.Disabled(len(a.designFuture) == 0 && len(a.asterDraftRedo) == 0)
			if redo.Clicked() {
				a.designRedo()
			}
			if a.createRunErr != "" {
				ui.Text(c, a.createRunErr).FontSize(11).TextColor(t.Danger).
					SingleLine().Ellipsis("…").MaxWidth(360)
			}
			if components.Button(c, "创建 Run", components.ButtonPrimary).Clicked() {
				a.preflightRisk = false
				a.preflightOpen = true
			}
			renderAgentProposal(c,a)
		})
		if a.designDocErr != "" {
			ui.Row(c).FillWidth().Padding(6, 12).Background(theme.GetTokens(t).DangerBg).Children(func() {
				ui.Text(c, a.designDocErr).FontSize(11).TextColor(theme.GetTokens(t).DangerText)
			})
		}
		if a.designTab == 0 {
			a.renderWorkflowCanvas(c)
			a.renderPreflight(c)
			return
		}
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).FillWidth().Padding(t.Space(4), t.Space(5)).
				AlignItems(ui.Center).Children(func() {
				ui.Box(c).FillWidth().MaxWidth(980).MaxWidthPercent(100).
					Background(t.Surface).Border(1, t.Border).Radius(t.Space(2)).
					Padding(t.Space(5), t.Space(6)).Children(func() {
					if a.designView == nil || a.designView.Doc == nil || a.designView.Doc.Source != a.designDoc.Source {
						a.designView = &asterdoc.View{
							Doc: doc, Mode: asterdoc.ModeDesign, DocumentRevision: fmt.Sprint(a.designDoc.Revision),
							Allowed: map[string]bool{asterdoc.PermDocumentEdit: true},
							OnAssignWells: func(moduleID, group string, wells []string) {
								a.applyDesignCommand("assign_wells", moduleID, group, wells)
							},
							OnClearWells: func(moduleID string, wells []string) {
								a.applyDesignCommand("clear_wells", moduleID, "", wells)
							},
							OnUpdateModule: func(moduleID string, props map[string]any) {
								_ = a.applyDesignRaw(map[string]any{
									"type": "update_module", "moduleId": moduleID, "properties": props,
								})
							},
						}
					}
					a.designView.Render(c)
				})
			})
		})
	})
	a.renderPreflight(c)
	a.captureAsterDraft()
}
