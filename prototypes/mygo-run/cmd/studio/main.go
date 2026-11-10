// Command studio previews the workflow card in isolation: every state of every
// step kind, drawn with the same Onevra materials the Designer uses.
package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"

	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/uikit/workflownode"
)

type studio struct{ dark bool }

var fixtures = []workflownode.NodeProps{
	{ID: "fixture-material", Kind: workflownode.KindMaterial, Order: "01", Title: "示例材料节点", Params: []workflownode.ParamItem{{Key: "样本", Val: "示例值"}}},
	{ID: "fixture-operation", Kind: workflownode.KindOperation, Order: "02", Title: "示例操作节点", Params: []workflownode.ParamItem{{Key: "参数", Val: "示例值"}}},
	{ID: "fixture-instrument", Kind: workflownode.KindInstrument, Order: "03", Title: "示例仪器节点"},
}

var states = []workflownode.NodeState{
	workflownode.StateDefault, workflownode.StateHover, workflownode.StateSelected,
	workflownode.StateError, workflownode.StateDisabled,
}

func (s *studio) View(c *ui.Context) {
	c.SetTheme(theme.Palette(s.dark))
	t := c.Theme()
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(t.Space(6)).Gap(t.Space(5)).Background(t.Background).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(t.Space(3)).Children(func() {
				components.Text(c, "WorkflowNode").FontSize(t.Rem(1.4)).FontWeight(600)
				components.Text(c, "Onevra 卡片 · 五种状态").TextColor(t.TextMuted)
				ui.Spacer(c)
				label := "深色"
				if s.dark {
					label = "浅色"
				}
				if components.Button(c, label, components.ButtonSecondary).Clicked() {
					s.dark = !s.dark
				}
			})
			components.Text(c, "以下均为隔离的展示 fixture，不是生产实验数据。").
				TextColor(t.TextMuted).FontSize(t.Rem(.85))
			for _, state := range states {
				ui.Column(c).Gap(t.Space(2)).Children(func() {
					components.Text(c, labelFor(state)).FontSize(t.Rem(.78)).TextColor(t.TextMuted)
					ui.Row(c).Wrap().Gap(t.Space(4)).Children(func() {
						for _, fixture := range fixtures {
							props := fixture
							props.State = state
							workflownode.View(c, props, 1, nil)
						}
					})
				})
			}
		})
	})
}

func labelFor(state workflownode.NodeState) string {
	switch state {
	case workflownode.StateHover:
		return "悬停"
	case workflownode.StateSelected:
		return "已选择"
	case workflownode.StateError:
		return "错误"
	case workflownode.StateDisabled:
		return "不可用"
	}
	return "默认"
}

func main() {
	s := &studio{}
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
			Title:         "WorkflowNode Component Studio",
			Width:         1040,
			Height:        720,
			TitleBarStyle: mygo.TitleBarHidden,
			Content:       ui.View(s.View),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
