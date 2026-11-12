package main

// Protocol is surface 6: a read-only projection of a plan. It renders two
// different objects and must never conflate them (docs/product/ux-contract.md
// §1.1): a Run-backed document is 已冻结, a design-only projection is 预览.

import (
	"fmt"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/protocol"
	"github.com/egoist/mygo/ui"
)

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func protocolPage(c *ui.Context, a *app) {
	// A document-backed Run reads its exact frozen AsterDoc revision.
	if a.snap.RunID != "" && a.snap.AsterDoc != nil {
		ui.Column(c).Fill().Children(func() {
			ui.Row(c).FillWidth().Padding(c.Theme().Space(2), c.Theme().Space(4)).
				AlignItems(ui.Center).Children(func() {
				ui.Text(c, fmt.Sprintf("已冻结 · Run %s · rev %d",
					shortID(a.snap.RunID), a.snap.AsterDoc.Revision)).
					FontSize(11).Bold().TextColor(c.Theme().TextMuted)
				ui.Spacer(c)
				a.documentExportButton(c)
			})
			if a.asterErr != "" {
				ui.Column(c).Grow(1).Padding(c.Theme().Space(8)).Gap(c.Theme().Space(2)).Children(func() {
					ui.Text(c, "冻结文档无法读取。").FontSize(16).Bold().TextColor(c.Theme().Danger)
					ui.Text(c, a.asterErr).FontSize(13).TextColor(c.Theme().TextMuted)
				})
				return
			}
			ui.Scroll(c).Fill().Children(func() {
				ui.Column(c).FillWidth().Padding(c.Theme().Space(6), c.Theme().Space(6)).
					AlignItems(ui.Center).Children(func() {
					ui.Box(c).FillWidth().MaxWidth(900).MaxWidthPercent(100).
						Radius(float32(18)).Border(1, c.Theme().Border).Background(c.Theme().Surface).
						Padding(c.Theme().Space(7), c.Theme().Space(8)).Children(func() {
						a.renderFrozenAsterDoc(c, asterdoc.ModeProtocol)
					})
				})
			})
		})
		return
	}
	// A loaded Run must read its frozen design. If that revision is missing or
	// corrupt the page reports it; showing the live draft would pass a later
	// edit off as the plan executed on the day.
	if a.snap.RunID != "" && a.snap.AsterDoc == nil {
		ui.Row(c).FillWidth().Padding(c.Theme().Space(2), c.Theme().Space(4)).Children(func() {
			ui.Text(c, fmt.Sprintf("已冻结（无文档） · Design rev %d", a.snap.DesignRevision)).
				FontSize(11).TextColor(c.Theme().TextMuted)
		})
	}
	if a.snap.RunID != "" {
		if a.snap.DesignData == "" {
			ui.Column(c).Grow(1).Padding(c.Theme().Space(8)).Gap(c.Theme().Space(2)).Children(func() {
				ui.Text(c, "已选择 Run，但它的冻结设计缺失。").FontSize(16).Bold().
					TextColor(c.Theme().Danger)
				ui.Text(c, "不会改用实时设计；请检查该运行的设计快照。").
					FontSize(13).TextColor(c.Theme().TextMuted)
			})
			return
		}
		doc, err := protocol.Build(a.snap.DesignData, a.snap.DesignID, a.snap.DesignRevision)
		if err != nil {
			ui.Text(c, "冻结设计无法生成实验方案："+err.Error()).TextColor(c.Theme().Danger)
			return
		}
		protocol.ViewDocument(c, doc)
		return
	}
	// No Run yet: this is a preview of an explicit design revision, not a
	// frozen record, and it cannot be exported as one.
	state := &protocol.State{DesignID: a.shell.Selected, Revision: a.snap.DesignRevision}
	if a.designer != nil {
		state.Document = a.designer.Editor.Doc
		state.Instances = a.designer.Instances
		state.Revision = a.designer.Revision
	}
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).FillWidth().Padding(c.Theme().Space(2), c.Theme().Space(4)).
			AlignItems(ui.Center).Children(func() {
			ui.Text(c, fmt.Sprintf("预览 · Design rev %d", state.Revision)).
				FontSize(11).Bold().TextColor(c.Theme().TextMuted)
			ui.Text(c, "未创建 Run；这是设计修订的只读预览，不是冻结记录。").
				FontSize(11).TextColor(c.Theme().TextMuted)
		})
		protocol.View(c, state)
	})
}
