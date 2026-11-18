package designer

import (
	"context"
	"fmt"
	"strings"

	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/workflow"
	"asterlab-mygo-prototype/modules/runs"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
)

// Section is the Design surface's content switch. Switching never touches the
// selected step or the draft: the section is view state, exactly like canvas
// pan/zoom, so UX-P5 "state is recoverable" holds without extra plumbing.
type Section uint8

const (
	SectionFlow Section = iota
	SectionPlate
	SectionDilution
)

func (s Section) String() string {
	switch s {
	case SectionPlate:
		return "12 孔板"
	case SectionDilution:
		return "配液单"
	}
	return "流程"
}

// stepCardView is one row of the design's step list. Every field is derived
// from the design the Designer already holds; nothing here is persisted.
type stepCardView struct {
	ID         string
	Order      string
	Title      string
	TypeTag    string
	Params     []string
	Assay      string
	Timepoints string
	// ProtocolPath and the ids below are the technical bindings UX-P2 keeps
	// folded until the researcher opens them.
	ProtocolPath  string
	References    []string
	BlockVersions []string

	totalMin, handsOnMin int
	hasTotal, hasHandsOn bool
}

// summary is the structured parameter line the card shows under its title.
// It prefers the step's own recorded parameters and never prints a
// "未填写参数" placeholder: with nothing recorded it falls back to the assay
// and timepoints the step actually claims, and to nothing at all after that.
func (v stepCardView) summary() string {
	if len(v.Params) > 0 {
		return strings.Join(v.Params, " · ")
	}
	var parts []string
	if v.Assay != "" {
		parts = append(parts, "检测 "+v.Assay)
	}
	if v.Timepoints != "" {
		parts = append(parts, "时间点 "+v.Timepoints)
	}
	return strings.Join(parts, " · ")
}

// durationLabel is the card's cost tag, e.g. "占手 45m" or "耗时 30m".
func (v stepCardView) durationLabel() string {
	switch {
	case v.hasHandsOn:
		return "占手 " + formatMinutes(v.handsOnMin)
	case v.hasTotal:
		return "耗时 " + formatMinutes(v.totalMin)
	}
	return ""
}

type designBudget struct {
	Steps, Wells                 int
	TotalMinutes, HandsOnMinutes int
	HasTotal, HasHandsOn         bool
	Core                         runs.TimeBudget
	HasCore                      bool
	CoreErr                      error
	// Unknown counts steps without usable timing; Problems carries the
	// step-scoped cause so the bar never estimates over gaps.
	Unknown  int
	Problems []string
}

func (d *Designer) designBudget() designBudget {
	b := designBudget{
		Steps: len(d.Editor.Doc.Nodes),
		Wells: uniqueWellCount(d.Assays[d.AssayIDDraft].Wells),
	}
	for _, card := range d.stepCardViews() {
		if card.hasTotal {
			b.TotalMinutes += card.totalMin
			b.HasTotal = true
		}
		if card.hasHandsOn {
			b.HandsOnMinutes += card.handsOnMin
			b.HasHandsOn = true
		}
	}
	// Timing inputs are the only feed into Core's projection: steps without
	// usable timing are counted unknown, never estimated. The cache is
	// invalidated by Touch, Save and Load.
	input, known, unknown, problems := BudgetInput(d.Editor.Doc.Nodes)
	_ = known
	if !d.budgetLoaded {
		d.budgetLoaded = true
		d.budget = runs.TimeBudget{}
		d.budgetErr = nil
		d.budgetUnknown = unknown
		d.budgetProblems = problems
		if d.TimeBudgetSvc == nil {
			message := "耗时估算服务暂不可用"
			if d.TimeBudgetError != "" {
				message += ": " + d.TimeBudgetError
			}
			d.budgetErr = fmt.Errorf("%s", message)
		} else {
			d.budget, d.budgetErr = d.TimeBudgetSvc.ProjectTimeBudget(context.Background(),
				runs.ProjectTimeBudgetInput{SpecsJSON: input})
		}
	}
	b.Core, b.CoreErr = d.budget, d.budgetErr
	b.Unknown, b.Problems = d.budgetUnknown, d.budgetProblems
	b.HasCore = b.CoreErr == nil && (b.Core.TotalSpanS > 0 || b.Core.HandsOnS > 0 || len(b.Core.ParallelWindows) > 0)
	return b
}

// stepCardViews derives one card per workflow node in document order.
func (d *Designer) stepCardViews() []stepCardView {
	cards := make([]stepCardView, 0, len(d.Editor.Doc.Nodes))
	for i := range d.Editor.Doc.Nodes {
		node := d.Editor.Doc.Nodes[i]
		card := stepCardView{
			ID:         node.ID,
			Title:      node.Title,
			Assay:      d.StepAssays[node.ID],
			Timepoints: strings.Join(d.StepTimepoints[node.ID], ", "),
			TypeTag:    node.Tag,
			References: append([]string(nil), node.ComponentRefs...),
		}
		if card.TypeTag == "" {
			card.TypeTag = kindTag(node.Kind)
		}
		card.Order = node.Order
		if card.Order == "" {
			card.Order = fmt.Sprintf("%02d", i+1)
		}
		for _, param := range node.Params {
			key, val := strings.TrimSpace(param.Key), strings.TrimSpace(param.Val)
			if val == "" {
				continue
			}
			if key != "" {
				card.Params = append(card.Params, key+" "+val)
			} else {
				card.Params = append(card.Params, val)
			}
			if !isDurationKey(key) {
				continue
			}
			minutes, ok := parseMinutes(val)
			if !ok {
				continue
			}
			if isHandsOnKey(key) {
				card.handsOnMin += minutes
				card.hasHandsOn = true
			} else {
				card.totalMin += minutes
				card.hasTotal = true
			}
		}
		if card.ProtocolPath == "" {
			for _, timepoint := range d.StepTimepoints[node.ID] {
				if path := d.StepTaskPaths[stepTaskKey(node.ID, timepoint)]; path != "" {
					card.ProtocolPath = path
					break
				}
			}
		}
		for _, inst := range d.Instances[node.ID] {
			card.BlockVersions = append(card.BlockVersions,
				fmt.Sprintf("%s@%d", inst.DefinitionID, inst.DefinitionVersion))
		}
		cards = append(cards, card)
	}
	return cards
}

func kindTag(kind workflow.NodeKind) string {
	switch kind {
	case workflow.KindFlask:
		return "材料 · 样本"
	case workflow.KindPlate:
		return "分析仪器"
	}
	return "操作步骤"
}

// isDurationKey narrows duration parsing to keys that actually describe time
// cost. A "时间点" is a planned checkpoint, not a duration, so it is excluded.
func isDurationKey(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	if k == "" || strings.Contains(k, "时间点") || strings.Contains(k, "timepoint") {
		return false
	}
	for _, needle := range []string{"耗时", "时长", "duration", "占手", "hands-on", "手占", "incubat", "时间"} {
		if strings.Contains(k, needle) {
			return true
		}
	}
	return false
}

func isHandsOnKey(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	for _, needle := range []string{"占手", "hands", "手占", "人工"} {
		if strings.Contains(k, needle) {
			return true
		}
	}
	return false
}

// parseMinutes reads a duration written the way lab notes write it: "5m",
// "45 min", "1.5h", "90 分钟", "2 小时". Values without a time unit are not
// durations and stay unconverted.
func parseMinutes(raw string) (int, bool) {
	s := strings.TrimSpace(strings.ToLower(raw))
	if s == "" {
		return 0, false
	}
	i := 0
	for i < len(s) && (s[i] >= '0' && s[i] <= '9' || s[i] == '.') {
		i++
	}
	if i == 0 {
		return 0, false
	}
	var n float64
	if _, err := fmt.Sscanf(s[:i], "%g", &n); err != nil {
		return 0, false
	}
	rest := strings.TrimSpace(s[i:])
	switch {
	case strings.HasPrefix(rest, "h"), strings.Contains(rest, "小时"), strings.HasSuffix(rest, "时"):
		return int(n*60 + 0.5), true
	case strings.HasPrefix(rest, "m"), strings.Contains(rest, "分钟"), strings.HasSuffix(rest, "分"):
		return int(n + 0.5), true
	case strings.HasPrefix(rest, "s"), strings.Contains(rest, "秒"):
		return int(n/60 + 0.5), true
	}
	return 0, false
}

func formatMinutes(minutes int) string {
	if minutes <= 0 {
		return "—"
	}
	h, m := minutes/60, minutes%60
	switch {
	case h == 0:
		return fmt.Sprintf("%dm", m)
	case m == 0:
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh%dm", h, m)
}

func formatBudget(minutes int, known bool) string {
	if !known {
		return "—"
	}
	return formatMinutes(minutes)
}

// ListView is the Design surface: one budget bar, one section switch and one
// vertical list of step cards. The workflow canvas stays available behind the
// 画布 toggle; it is not deleted, only demoted from the default view.
func ListView(c *ui.Context, d *Designer) {
	c.OnShortcut(ui.Cmd, ui.KeyS, func() {
		d.Save(context.Background())
	})
	c.OnShortcut(ui.Cmd, ui.KeyK, func() {
		d.CanvasMode = true
	})

	ui.Column(c).Fill().Background(c.Theme().Background).Children(func() {
		listToolbar(c, d)
		listErrorBanner(c, d)
		BudgetBar(c, d)
		sectionSwitch(c, d)
		ui.Box(c).Fill().Grow(1).MinHeight(0).Children(func() {
			switch d.Section {
			case SectionPlate:
				listSection(c, func() { plateModule(c, d) })
			case SectionDilution:
				listSection(c, func() {
					modulesSection(c, d)
					assayInspector(c, d)
					dilutionTotalsSection(c, d)
				})
			default:
				stepCardList(c, d)
			}
		})
		statusBar(c, d)
	})
	confirmCreateRun(c, d)
	d.captureDraft()
}

func listToolbar(c *ui.Context, d *Designer) {
	t := c.Theme()
	ui.Row(c).FillWidth().Wrap().Padding(t.Space(2), t.Space(4)).Background(t.Surface).
		Border(1, t.Border).AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
		components.Text(c, d.listTitle()).FontSize(t.Rem(1.05)).FontWeight(600).TextColor(t.Text)
		components.Text(c, fmt.Sprintf("rev %d", d.Revision)).FontSize(t.Rem(.75)).TextColor(t.TextMuted)
		listStateChip(c, d)
		ui.Spacer(c)
		if components.IconButton(c, lucide.Workflow(), "画布视图", components.IconButtonCompact).Clicked() {
			d.CanvasMode = true
		}
		if components.IconButton(c, lucide.Save(), "保存", components.IconButtonCompact).Clicked() {
			d.Save(context.Background())
		}
		createBtn := components.Button(c, "创建 Run", components.ButtonPrimary)
		createBtn.Disabled(d.Loading)
		if createBtn.Clicked() && !d.Loading {
			d.RequestCreateRun(context.Background())
		}
	})
}

func listErrorBanner(c *ui.Context, d *Designer) {
	if d.Error == "" {
		return
	}
	t := c.Theme()
	ui.Row(c).FillWidth().Padding(t.Space(1.5), t.Space(4)).
		Background(t.Danger.Alpha(.08)).Border(1, t.Danger.Alpha(.25)).
		AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
		ui.Box(c).Grow(1).MinWidth(0).Children(func() {
			components.Text(c, "未保存 · "+d.Error).TextColor(t.Danger).FontSize(t.Rem(.78))
		})
		if components.Button(c, "重试", components.ButtonSecondary).Clicked() {
			d.retryListError(context.Background())
		}
	})
}

func (d *Designer) retryListError(ctx context.Context) {
	if d.Dirty {
		d.Save(ctx)
		return
	}
	if d.Revision == 0 && d.DesignID != "" {
		d.Load(ctx, d.DesignID)
		return
	}
	d.RequestCreateRun(ctx)
}

func (d *Designer) listTitle() string {
	if title := strings.TrimSpace(d.Title); title != "" {
		return title
	}
	return "实验设计"
}

// listStateChip names the design's state in the §3 vocabulary. The word is
// the same one the canvas toolbar's save badge uses.
func listStateChip(c *ui.Context, d *Designer) {
	switch {
	case d.Loading:
		components.Badge(c, "校验中", components.BadgeOptions{Tone: components.BadgeNeutral})
	case d.SaveState == SaveSaving:
		components.Badge(c, "正在保存…", components.BadgeOptions{Tone: components.BadgeNeutral})
	case d.SaveState == SaveError:
		components.Badge(c, "未保存", components.BadgeOptions{Tone: components.BadgeDanger})
	case d.Dirty:
		components.Badge(c, "未保存", components.BadgeOptions{Tone: components.BadgeWarning})
	case d.SaveState == SaveSaved:
		components.Badge(c, "已保存", components.BadgeOptions{Tone: components.BadgeSuccess})
	case d.Empty:
		components.Badge(c, "草稿", components.BadgeOptions{Tone: components.BadgeNeutral})
	default:
		components.Badge(c, "草稿", components.BadgeOptions{Tone: components.BadgeNeutral})
	}
}

// BudgetBar renders Core time when available and separately labeled design annotations otherwise.
func BudgetBar(c *ui.Context, d *Designer) {
	t := c.Theme()
	tokens := theme.GetTokens(t)
	b := d.designBudget()
	chip := func(label, value string) {
		ui.Row(c).Gap(float32(tokens.Spacing4)).AlignItems(ui.Center).Children(func() {
			components.Text(c, label).FontSize(float32(tokens.TextMeta)).TextColor(tokens.TextMuted)
			components.Text(c, value).FontSize(float32(tokens.TextBody)).FontWeight(600).
				TextColor(tokens.Text).FontFeatures("tnum")
		})
	}
	ui.Row(c).FillWidth().Wrap().Padding(float32(tokens.Spacing8), float32(tokens.Spacing16)).
		Background(tokens.Canvas).AlignItems(ui.Center).Gap(float32(tokens.Spacing16)).Children(func() {
		if b.HasCore {
			chip("来源", "方案耗时测算")
			chip("总跨度", formatBudgetSeconds(b.Core.TotalSpanS, true))
			chip("净占手", formatBudgetSeconds(b.Core.HandsOnS, true))
			chip("可并行空窗", formatBudgetWindows(b.Core.ParallelWindows))
		} else {
			chip("来源", "设计标注")
			chip("总跨度", formatBudget(b.TotalMinutes, b.HasTotal))
			chip("净占手", formatBudget(b.HandsOnMinutes, b.HasHandsOn))
			chip("可并行空窗", "—")
			if b.CoreErr != nil {
				components.Text(c, "耗时测算暂不可用："+b.CoreErr.Error()).
					FontSize(float32(tokens.TextMeta)).TextColor(tokens.Danger)
			} else {
				components.Text(c, "当前步骤尚未配置预计耗时").
					FontSize(float32(tokens.TextMeta)).TextColor(tokens.TextMuted)
			}
		}
		// Unknowns are named, never estimated: a step without usable timing
		// contributes nothing to Core's projection.
		if b.Unknown > 0 {
			chip("未知", fmt.Sprintf("%d 步", b.Unknown))
		}
		if len(b.Problems) > 0 {
			components.Text(c, b.Problems[0]).
				FontSize(float32(tokens.TextMeta)).TextColor(tokens.Danger)
		}
		chip("步骤", fmt.Sprintf("%d", b.Steps))
		if b.Wells > 0 {
			chip("孔位", fmt.Sprintf("%d", b.Wells))
		}
		if b.CoreErr != nil {
			if components.Button(c, "重试预算", components.ButtonSecondary).Clicked() {
				d.budgetLoaded = false
				d.designBudget()
			}
		}
	})
}

func formatBudgetSeconds(seconds float64, known bool) string {
	if !known || seconds < 0 {
		return "—"
	}
	total := int64(seconds + .5)
	hours, minutes, secs := total/3600, total%3600/60, total%60
	switch {
	case hours > 0 && minutes > 0:
		return fmt.Sprintf("%dh%dm", hours, minutes)
	case hours > 0:
		return fmt.Sprintf("%dh", hours)
	case minutes > 0 && secs > 0:
		return fmt.Sprintf("%dm%ds", minutes, secs)
	case minutes > 0:
		return fmt.Sprintf("%dm", minutes)
	default:
		return fmt.Sprintf("%ds", secs)
	}
}

func formatBudgetWindows(windows []runs.TimeBudgetWindow) string {
	if len(windows) == 0 {
		return "—"
	}
	labels := make([]string, 0, len(windows))
	for _, window := range windows {
		label := fmt.Sprintf("步骤%02d %s (%s–%s)", window.StepIndex+1,
			formatBudgetSeconds(window.DurationS, true),
			formatBudgetSeconds(window.StartS, true), formatBudgetSeconds(window.EndS, true))
		if window.TimeAnchorID != "" {
			label += " · " + window.TimeAnchorID
		}
		labels = append(labels, label)
	}
	return strings.Join(labels, "；")
}

// sectionSwitch is the 流程 / 12 孔板 / 配液单 segmented control. Choosing a
// section only changes what the main region draws.
func sectionSwitch(c *ui.Context, d *Designer) {
	t := c.Theme()
	index := int(d.Section)
	ui.Row(c).FillWidth().Padding(0, t.Space(4), t.Space(1.5), t.Space(4)).Children(func() {
		components.Tabs(c, &index, components.TabsSegmented,
			SectionFlow.String(), SectionPlate.String(), SectionDilution.String())
	})
	d.Section = Section(index)
}

// listSection wraps a secondary section (plate or dilution) in a scroll.
func listSection(c *ui.Context, build func()) {
	t := c.Theme()
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).FillWidth().Padding(t.Space(3), t.Space(4)).Gap(t.Space(3)).
			Children(build)
	})
}

func stepCardList(c *ui.Context, d *Designer) {
	t := c.Theme()
	cards := d.stepCardViews()
	if len(cards) == 0 {
		ui.Column(c).Fill().Center().Gap(t.Space(2)).Children(func() {
			components.Text(c, "还没有步骤").FontWeight(600)
			components.Text(c, "切换回画布，用上方的工具添加材料、操作或仪器步骤。").
				TextColor(t.TextMuted).FontSize(t.Rem(.85)).TextAlign(ui.Center)
		})
		return
	}
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).FillWidth().Padding(t.Space(3), t.Space(4)).Gap(t.Space(2)).
			Children(func() {
				for _, card := range cards {
					stepCard(c, d, card)
				}
			})
	})
}

func stepCard(c *ui.Context, d *Designer, card stepCardView) {
	t := c.Theme()
	selected := d.SelectedNode == card.ID
	border := t.Border
	if selected {
		border = t.Accent
	}
	ui.Box(c).FillWidth().Key("step-" + card.ID).Children(func() {
		ui.Column(c).FillWidth().Padding(t.Space(3), t.Space(3)).Gap(t.Space(1.5)).
			Background(t.Surface).Border(1, border).Radius(t.Space(1.5)).Children(func() {
			ui.Row(c).FillWidth().MinWidth(0).AlignItems(ui.Center).Gap(t.Space(1.5)).
				OnClick(func() { d.selectStep(card.ID) }).Children(func() {
				orderChip(c, card.Order, selected)
				components.Text(c, orStepTitle(card.Title)).FontSize(t.Rem(.95)).
					FontWeight(600).TextColor(t.Text).SingleLine().Ellipsis("…").Grow(1)
				if card.TypeTag != "" {
					components.Badge(c, card.TypeTag, components.BadgeOptions{Tone: components.BadgeNeutral})
				}
				if label := card.durationLabel(); label != "" {
					components.Badge(c, label, components.BadgeOptions{Tone: components.BadgeAccent})
				}
			})
			if summary := card.summary(); summary != "" {
				components.Text(c, summary).FontSize(t.Rem(.8)).TextColor(t.TextMuted).
					SingleLine().Ellipsis("…")
			}
			cardProperties(c, d, card)
		})
	})
}

// orderChip is the card's 序号: a small outlined square, the same weight as
// workflownode's order label.
func orderChip(c *ui.Context, order string, selected bool) {
	t := c.Theme()
	fg, edge := t.TextMuted, t.Border
	if selected {
		fg, edge = t.Accent, t.Accent
	}
	ui.Box(c).Size(t.Space(6), t.Space(6)).Shrink(0).Center().Radius(t.Space(1)).
		Background(t.Background).Border(1, edge).Children(func() {
		components.Text(c, order).FontSize(t.Rem(.72)).FontWeight(600).TextColor(fg).
			FontFeatures("tnum")
	})
}

// cardProperties is the UX-P2 disclosure: protocol path, reference ids and
// block-version ids are built only while it is open, so the default view never
// shows a raw identifier.
func cardProperties(c *ui.Context, d *Designer, card stepCardView) {
	t := c.Theme()
	if d.PropertiesOpen == nil {
		d.PropertiesOpen = map[string]*bool{}
	}
	open := d.PropertiesOpen[card.ID]
	if open == nil {
		closed := false
		open = &closed
		d.PropertiesOpen[card.ID] = open
	}
	components.Collapsible(c, "技术属性", open, components.CollapsibleOptions{
		Trailing: func() {
			components.Text(c, bindingsHint(card)).FontSize(t.Rem(.7)).TextColor(t.TextMuted)
		},
	}, func() {
		propertyRow(c, "Protocol path", orMissing(card.ProtocolPath))
		propertyRow(c, "引用 ID", orMissing(strings.Join(card.References, ", ")))
		propertyRow(c, "模块版本", orMissing(strings.Join(card.BlockVersions, ", ")))
	})
}

func propertyRow(c *ui.Context, label, value string) {
	t := c.Theme()
	ui.Row(c).FillWidth().MinWidth(0).Gap(t.Space(2)).Children(func() {
		ui.Box(c).Width(t.Space(24)).Shrink(0).Children(func() {
			components.Text(c, label).FontSize(t.Rem(.72)).TextColor(t.TextMuted)
		})
		components.Text(c, value).FontSize(t.Rem(.72)).TextColor(t.Text).
			SingleLine().Ellipsis("…")
	})
}

// bindingsHint counts the technical bindings the disclosure holds, so the
// folded row still says what is behind it.
func bindingsHint(card stepCardView) string {
	count := 0
	if card.ProtocolPath != "" {
		count++
	}
	count += len(card.References) + len(card.BlockVersions)
	if count == 0 {
		return "无技术绑定"
	}
	return fmt.Sprintf("%d 项绑定", count)
}

func orStepTitle(title string) string {
	if title = strings.TrimSpace(title); title != "" {
		return title
	}
	return "未命名步骤"
}

func orMissing(value string) string {
	if strings.TrimSpace(value) == "" {
		return "无"
	}
	return value
}

// selectStep focuses a step from the list. It routes through the editor so the
// same selection callbacks (assay draft, timepoints) run as on the canvas.
func (d *Designer) selectStep(id string) {
	if d.SelectedNode == id {
		return
	}
	d.Editor.SelectNode(id)
}
