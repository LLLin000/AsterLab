package tools

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/uikit/kit"
	"asterlab-mygo-prototype/internal/uikit/labsticker"
)

// WeightEntry records a dated body-weight measurement for ARRIVE 2.0 & welfare tracking.
type WeightEntry struct {
	Date    string  `json:"date"`
	WeightG float64 `json:"weightG"`
	Note    string  `json:"note,omitempty"`
}

// VivariumCage models one animal housing cage / aquarium unit backed by
// IACUC compliance, ARRIVE 2.0 genetic/census fields, and GB 14925-2023 density rules.
type VivariumCage struct {
	ID            string                     `json:"id"`
	CageCode      string                     `json:"cageCode"`
	Species       labsticker.AnimalSpeciesID `json:"species"`
	Strain        string                     `json:"strain"`
	Genotype      string                     `json:"genotype,omitempty"`
	MaleCount     int                        `json:"maleCount"`
	FemaleCount   int                        `json:"femaleCount"`
	FacilityGrade string                     `json:"facilityGrade"` // SPF | conventional | isolator | aquatic
	Room          string                     `json:"room"`
	RackPos       string                     `json:"rackPos"`
	DOB           string                     `json:"dob,omitempty"`
	WeightG       float64                    `json:"weightG,omitempty"`
	WeightHistory []WeightEntry              `json:"weightHistory,omitempty"`
	IACUCNo       string                     `json:"iacucNo,omitempty"`
	Vendor        string                     `json:"vendor,omitempty"`
	CertNo        string                     `json:"certNo,omitempty"`
	IDMethod      string                     `json:"idMethod,omitempty"`
	AnimalIDs     string                     `json:"animalIds,omitempty"`
	Status        string                     `json:"status"` // quarantine | active | in_study | breeding | archived
	ExperimentRef string                     `json:"experimentRef,omitempty"`
	HealthNotes   string                     `json:"healthNotes,omitempty"`
	UpdatedAt     string                     `json:"updatedAt,omitempty"`
}

// TotalAnimals returns total headcount (male + female) in the cage.
func (c VivariumCage) TotalAnimals() int {
	m := max(0, c.MaleCount)
	f := max(0, c.FemaleCount)
	return m + f
}

// EnsureVivariumLoaded lazily loads saved cages from Callbacks.LoadVivarium.
func (w *Workspace) EnsureVivariumLoaded() {
	if w.vivariumLoaded {
		return
	}
	w.vivariumLoaded = true
	if w.Callbacks.LoadVivarium == nil {
		return
	}
	if cages, err := w.Callbacks.LoadVivarium(); err == nil && cages != nil {
		w.VivariumCages = cages
	}
}

func (w *Workspace) saveVivarium() error {
	if w.Callbacks.SaveVivarium != nil {
		return w.Callbacks.SaveVivarium(w.VivariumCages)
	}
	return nil
}

// VivariumAgeLabel computes live biological age (weeks or days) from YYYY-MM-DD DOB.
func VivariumAgeLabel(dob string, now time.Time) string {
	dob = strings.TrimSpace(dob)
	if dob == "" {
		return "—"
	}
	parsed, err := time.Parse("2006-01-02", dob)
	if err != nil {
		return dob
	}
	days := int(now.Sub(parsed).Hours() / 24)
	if days < 0 {
		return "0 日龄"
	}
	if days < 21 {
		return fmt.Sprintf("%d 日龄", days)
	}
	return fmt.Sprintf("%.1f 周龄", float64(days)/7.0)
}

// VivariumDensityWarning checks GB 14925-2023 recommended single-cage occupancy thresholds.
func VivariumDensityWarning(c VivariumCage) string {
	if c.Status == "archived" {
		return ""
	}
	spec, ok := labsticker.LookupAnimal(c.Species)
	if !ok || spec.DefaultMaxPerCage <= 0 {
		return ""
	}
	tot := c.TotalAnimals()
	if tot > spec.DefaultMaxPerCage {
		return fmt.Sprintf("超密度 %d/限%d", tot, spec.DefaultMaxPerCage)
	}
	return ""
}

func vivariumGradeLabel(grade string) string {
	switch strings.TrimSpace(grade) {
	case "conventional":
		return "普通实验环境"
	case "isolator":
		return "隔离器无菌区"
	case "aquatic":
		return "水生动物循环系统"
	default:
		return "SPF 屏障环境"
	}
}

func vivariumStatusInfo(status string) (string, components.BadgeTone) {
	switch strings.TrimSpace(status) {
	case "quarantine":
		return "检疫适应", components.BadgeWarning
	case "in_study":
		return "实验中", components.BadgeAccent
	case "breeding":
		return "繁育配对", components.BadgeSuccess
	case "archived":
		return "已终末归档", components.BadgeNeutral
	default:
		return "正常在养", components.BadgeSuccess
	}
}

// OpenNewVivariumDialog opens the modal form to register a new animal cage card.
func (w *Workspace) OpenNewVivariumDialog() {
	w.EnsureVivariumLoaded()
	idx := len(w.VivariumCages) + 1
	w.vivariumEditingID = ""
	w.vivariumDialogErr = ""
	w.vivariumHoldDelReady = false
	w.vivariumDraft = VivariumCage{
		CageCode:      fmt.Sprintf("C-%02d", idx),
		Species:       labsticker.AnimalMouseBlack,
		Strain:        "C57BL/6J",
		Genotype:      "WT",
		MaleCount:     4,
		FemaleCount:   0,
		FacilityGrade: "SPF",
		Room:          "SPF-01 房",
		Status:        "active",
		IDMethod:      "剪耳标",
	}
	w.vivariumMaleF = float64(w.vivariumDraft.MaleCount)
	w.vivariumFemaleF = float64(w.vivariumDraft.FemaleCount)
	w.vivariumWeightF = 20.0
	w.VivariumDialogOpen = true
}

// OpenEditVivariumDialog loads an existing cage card into the editor dialog.
func (w *Workspace) OpenEditVivariumDialog(c VivariumCage) {
	w.EnsureVivariumLoaded()
	copyCage := c
	if len(c.WeightHistory) > 0 {
		copyCage.WeightHistory = append([]WeightEntry(nil), c.WeightHistory...)
	}
	w.vivariumEditingID = c.ID
	w.vivariumDialogErr = ""
	w.vivariumHoldDelReady = false
	w.vivariumDraft = copyCage
	w.vivariumMaleF = float64(copyCage.MaleCount)
	w.vivariumFemaleF = float64(copyCage.FemaleCount)
	w.vivariumWeightF = copyCage.WeightG
	w.VivariumDialogOpen = true
}

// SaveVivariumDraft validates and persists the current cage card draft.
func (w *Workspace) SaveVivariumDraft() bool {
	w.EnsureVivariumLoaded()
	w.vivariumDraft.CageCode = strings.TrimSpace(w.vivariumDraft.CageCode)
	w.vivariumDraft.Strain = strings.TrimSpace(w.vivariumDraft.Strain)
	if w.vivariumDraft.CageCode == "" {
		w.vivariumDialogErr = "请输入笼位编号（如 C-01）"
		return false
	}
	if w.vivariumDraft.Strain == "" {
		w.vivariumDialogErr = "请输入品系名称（如 C57BL/6J、SD 大鼠）"
		return false
	}
	if w.vivariumDraft.Species == "" {
		w.vivariumDraft.Species = labsticker.SuggestAnimalID(w.vivariumDraft.Strain)
	}
	w.vivariumDraft.MaleCount = max(0, int(w.vivariumMaleF+0.5))
	w.vivariumDraft.FemaleCount = max(0, int(w.vivariumFemaleF+0.5))
	w.vivariumDraft.WeightG = w.vivariumWeightF
	w.vivariumDraft.Genotype = strings.TrimSpace(w.vivariumDraft.Genotype)
	w.vivariumDraft.Room = strings.TrimSpace(w.vivariumDraft.Room)
	w.vivariumDraft.RackPos = strings.TrimSpace(w.vivariumDraft.RackPos)
	w.vivariumDraft.DOB = strings.TrimSpace(w.vivariumDraft.DOB)
	w.vivariumDraft.IACUCNo = strings.TrimSpace(w.vivariumDraft.IACUCNo)
	w.vivariumDraft.Vendor = strings.TrimSpace(w.vivariumDraft.Vendor)
	w.vivariumDraft.CertNo = strings.TrimSpace(w.vivariumDraft.CertNo)
	w.vivariumDraft.IDMethod = strings.TrimSpace(w.vivariumDraft.IDMethod)
	w.vivariumDraft.AnimalIDs = strings.TrimSpace(w.vivariumDraft.AnimalIDs)
	w.vivariumDraft.ExperimentRef = strings.TrimSpace(w.vivariumDraft.ExperimentRef)
	w.vivariumDraft.HealthNotes = strings.TrimSpace(w.vivariumDraft.HealthNotes)
	w.vivariumDraft.UpdatedAt = time.Now().Format("2006-01-02 15:04")

	if w.vivariumEditingID == "" {
		w.vivariumDraft.ID = fmt.Sprintf("viv-%d", time.Now().UnixNano())
		w.VivariumCages = append(w.VivariumCages, w.vivariumDraft)
	} else {
		replaced := false
		for i := range w.VivariumCages {
			if w.VivariumCages[i].ID == w.vivariumEditingID {
				w.vivariumDraft.ID = w.vivariumEditingID
				w.VivariumCages[i] = w.vivariumDraft
				replaced = true
				break
			}
		}
		if !replaced {
			w.vivariumDraft.ID = w.vivariumEditingID
			w.VivariumCages = append(w.VivariumCages, w.vivariumDraft)
		}
	}

	if err := w.saveVivarium(); err != nil {
		w.vivariumDialogErr = err.Error()
		return false
	}
	w.vivariumDialogErr = ""
	w.VivariumDialogOpen = false
	return true
}

func (w *Workspace) deleteVivariumCage(id string) {
	w.EnsureVivariumLoaded()
	next := w.VivariumCages[:0]
	for _, c := range w.VivariumCages {
		if c.ID != id {
			next = append(next, c)
		}
	}
	w.VivariumCages = next
	_ = w.saveVivarium()
	w.VivariumDialogOpen = false
}

func vivariumMatchesFilter(c VivariumCage, query, filter string) bool {
	switch filter {
	case "SPF":
		if c.FacilityGrade != "SPF" && c.FacilityGrade != "" {
			return false
		}
	case "in_study":
		if c.Status != "in_study" {
			return false
		}
	case "quarantine_breeding":
		if c.Status != "quarantine" && c.Status != "breeding" {
			return false
		}
	case "density_alert":
		if VivariumDensityWarning(c) == "" {
			return false
		}
	}
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return true
	}
	spec, _ := labsticker.LookupAnimal(c.Species)
	hay := strings.ToLower(strings.Join([]string{
		c.CageCode, spec.Title, c.Strain, c.Genotype, c.Room, c.RackPos,
		c.IACUCNo, c.Vendor, c.CertNo, c.AnimalIDs, c.ExperimentRef, c.HealthNotes,
	}, " "))
	return strings.Contains(hay, q)
}

func renderVivariumView(c *ui.Context, w *Workspace, tok theme.Tokens) {
	t := c.Theme()
	w.EnsureVivariumLoaded()

	// Aggregate stats
	totalCages := len(w.VivariumCages)
	totalAnimals := 0
	spfCages := 0
	inStudyCages := 0
	qbCages := 0
	densityAlerts := 0
	for _, cg := range w.VivariumCages {
		if cg.Status != "archived" {
			totalAnimals += cg.TotalAnimals()
		}
		if cg.FacilityGrade == "SPF" || cg.FacilityGrade == "" {
			spfCages++
		}
		if cg.Status == "in_study" {
			inStudyCages++
		}
		if cg.Status == "quarantine" || cg.Status == "breeding" {
			qbCages++
		}
		if VivariumDensityWarning(cg) != "" {
			densityAlerts++
		}
	}

	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
		kit.SectionHeader(c, tok, lucide.PawPrint(), "动物房 · 笼卡与实验动物台账",
			fmt.Sprintf("%d 笼 · %d 只/尾", totalCages, totalAnimals), func() {
				components.Tabs(c, &w.vivariumViewTab, components.TabsSegmented, "笼架视图", "动物台账")
				if components.Button(c, "+ 新建笼卡", components.ButtonPrimary).Clicked() {
					w.OpenNewVivariumDialog()
				}
			})
	})

	// Search & compliance filter bar
	ui.Row(c).FillWidth().Wrap().Padding(t.Space(2), t.Space(3)).
		Radius(float32(tok.PanelRadius)).Background(tok.Elevated).Border(1, tok.Border).
		AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
		ui.Box(c).Width(float32(tok.Spacing48)*5.4).Children(func() {
			components.TextBox(c, &w.vivariumSearch, components.TextBoxOptions{
				LeadingIcon: lucide.Search(),
			}).Placeholder("搜笼号、品系、基因型、伦理批件或实验…")
		})
		ui.Box(c).Width(1).Height(t.Space(4)).Background(tok.BorderStrong)
		filters := []struct {
			key, label string
		}{
			{"all", fmt.Sprintf("全部 (%d)", totalCages)},
			{"SPF", fmt.Sprintf("SPF 屏障 (%d)", spfCages)},
			{"in_study", fmt.Sprintf("实验中 (%d)", inStudyCages)},
			{"quarantine_breeding", fmt.Sprintf("检疫/繁育 (%d)", qbCages)},
			{"density_alert", fmt.Sprintf("密度预警 (%d)", densityAlerts)},
		}
		for _, f := range filters {
			f := f
			active := w.vivariumFilter == f.key || (w.vivariumFilter == "" && f.key == "all")
			variant := components.ButtonGhost
			if active {
				variant = components.ButtonToolbar
			}
			if components.Button(c, f.label, variant).FontFeatures("tnum").Clicked() {
				w.vivariumFilter = f.key
			}
		}
	})

	var filtered []VivariumCage
	for _, cg := range w.VivariumCages {
		if vivariumMatchesFilter(cg, w.vivariumSearch, w.vivariumFilter) {
			filtered = append(filtered, cg)
		}
	}

	if len(filtered) == 0 {
		msg := "点击右上角「+ 新建笼卡」登记小鼠、大鼠、兔、比格犬等实验动物笼位与 IACUC 批件。"
		if totalCages > 0 {
			msg = "当前搜索或筛选条件下无匹配笼卡，可切换「全部」查看。"
		}
		kit.EmptyState(c, tok, lucide.PawPrint(), "动物房暂无匹配笼卡", msg, "+ 新建笼卡", func() {
			w.OpenNewVivariumDialog()
		})
		return
	}

	if w.vivariumViewTab == 1 {
		renderVivariumTableView(c, w, tok, filtered)
		return
	}

	// Cage Card Rack View grouped by facility grade
	gradeOrder := []string{"SPF", "conventional", "isolator", "aquatic"}
	buckets := map[string][]VivariumCage{}
	for _, cg := range filtered {
		g := cg.FacilityGrade
		if g == "" {
			g = "SPF"
		}
		buckets[g] = append(buckets[g], cg)
	}

	now := time.Now()
	for _, g := range gradeOrder {
		list := buckets[g]
		if len(list) == 0 {
			continue
		}
		sort.SliceStable(list, func(i, j int) bool {
			return list[i].CageCode < list[j].CageCode
		})
		tierBox := ui.Column(c.Key("viv-tier-" + g)).FillWidth().
			Padding(float32(tok.Spacing12), float32(tok.Spacing16)).Gap(float32(tok.Spacing12))
		kit.ApplyElevation(tierBox, tok, kit.ElevationCard)
		tierBox.Children(func() {
			kit.SectionHeader(c, tok, lucide.Building(), vivariumGradeLabel(g),
				fmt.Sprintf("%d 笼", len(list)), nil)
			ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing12)).Children(func() {
				for _, cg := range list {
					cg := cg
					renderVivariumCageCard(c, w, tok, cg, now)
				}
			})
		})
	}
}

func renderVivariumCageCard(c *ui.Context, w *Workspace, tok theme.Tokens, cg VivariumCage, now time.Time) {
	spec, hasSpec := labsticker.LookupAnimal(cg.Species)
	unit := "只"
	if hasSpec && spec.DefaultUnit != "" {
		unit = spec.DefaultUnit
	}
	statusTxt, statusTone := vivariumStatusInfo(cg.Status)
	densityWarn := VivariumDensityWarning(cg)
	ageLabel := VivariumAgeLabel(cg.DOB, now)

	card := ui.ButtonBase(c.Key("viv-card-" + cg.ID)).
		Label("编辑笼卡 " + cg.CageCode + " " + cg.Strain).
		Width(float32(tok.Spacing48) * 6.2).
		Padding(float32(tok.Spacing12), float32(tok.Spacing12))
	kit.ApplyElevation(card, tok, kit.ElevationCard)
	if card.Hovered() {
		card.Border(1, tok.BorderStrong)
	}
	theme.ApplyTactilePress(card)
	kit.ApplyFocusRing(card, tok)

	card.Children(func() {
		ui.Column(c).FillWidth().Gap(float32(tok.Spacing8)).Children(func() {
			// Header: Cage code + Strain + Status badge
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Text(c, cg.CageCode).FontSize(float32(tok.TextData)).Bold().TextColor(tok.Accent).FontFeatures("tnum").SingleLine()
				ui.Text(c, cg.Strain).FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text).Grow(1).MinWidth(0).SingleLine().Ellipsis("…")
				components.Badge(c, statusTxt, components.BadgeOptions{Tone: statusTone})
			})

			// Body: Animal vector sticker + census / genotype / age / weight
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing12)).Children(func() {
				stickerBox := ui.Column(c).Padding(float32(tok.Spacing4)).
					Radius(float32(tok.ControlRadius)).
					Background(tok.Elevated).Border(1, tok.BorderSubtle).Center()
				stickerBox.Children(func() {
					labsticker.RenderAnimalSticker(c, cg.Species, 42)
				})
				ui.Column(c).Grow(1).MinWidth(0).Gap(float32(tok.Spacing4)).Children(func() {
					census := fmt.Sprintf("♂ %d · ♀ %d（共 %d %s）", cg.MaleCount, cg.FemaleCount, cg.TotalAnimals(), unit)
					ui.Text(c, census).FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text).FontFeatures("tnum").SingleLine()
					metaParts := []string{}
					if cg.Genotype != "" {
						metaParts = append(metaParts, "基因型 "+cg.Genotype)
					}
					if ageLabel != "—" {
						metaParts = append(metaParts, ageLabel)
					}
					if cg.WeightG > 0 {
						metaParts = append(metaParts, fmt.Sprintf("%.1f g", cg.WeightG))
					}
					if len(metaParts) > 0 {
						ui.Text(c, strings.Join(metaParts, " · ")).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).FontFeatures("tnum").SingleLine().Ellipsis("…")
					}
					loc := strings.TrimSpace(cg.Room)
					if cg.RackPos != "" {
						if loc != "" {
							loc += " · " + cg.RackPos
						} else {
							loc = cg.RackPos
						}
					}
					if loc == "" {
						loc = vivariumGradeLabel(cg.FacilityGrade)
					}
					ui.Text(c, loc).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine().Ellipsis("…")
				})
			})

			// Footer compliance badges: IACUC + GB 14925 density warning
			ui.Box(c).FillWidth().Height(1).Background(tok.BorderSubtle)
			ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
				if cg.IACUCNo != "" {
					components.Badge(c, "IACUC "+cg.IACUCNo, components.BadgeOptions{Tone: components.BadgeNeutral})
				} else {
					components.Badge(c, "待补伦理批件", components.BadgeOptions{Tone: components.BadgeWarning})
				}
				if densityWarn != "" {
					components.Badge(c, densityWarn, components.BadgeOptions{Tone: components.BadgeDanger})
				}
				if cg.ExperimentRef != "" {
					ui.Text(c, cg.ExperimentRef).FontSize(float32(tok.TextMeta)).TextColor(tok.Accent).SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
				}
			})
		})
	})

	if card.Clicked() {
		w.OpenEditVivariumDialog(cg)
	}
	kit.ContextMenu(c, card, func(m *kit.Menu) {
		if m.Item("编辑笼卡 / 记录体重").Chosen() {
			w.OpenEditVivariumDialog(cg)
		}
		if m.Item("标记为实验中").Chosen() {
			cg.Status = "in_study"
			w.OpenEditVivariumDialog(cg)
			w.SaveVivariumDraft()
		}
		m.Separator()
		if m.Item("删除笼卡").Chosen() {
			w.deleteVivariumCage(cg.ID)
		}
	})
}

func renderVivariumTableView(c *ui.Context, w *Workspace, tok theme.Tokens, filtered []VivariumCage) {
	now := time.Now()
	cols := []ui.TableColumn{
		{ID: "code", Title: "笼号", Width: 88},
		{ID: "strain", Title: "物种与品系", MinWidth: 160},
		{ID: "geno", Title: "基因型", Width: 108},
		{ID: "census", Title: "存栏 (♂/♀)", Width: 104, Align: ui.End},
		{ID: "age", Title: "周龄", Width: 84, Align: ui.End},
		{ID: "weight", Title: "体重 (g)", Width: 84, Align: ui.End},
		{ID: "loc", Title: "设施/位置", Width: 140},
		{ID: "iacuc", Title: "伦理批件", Width: 128},
		{ID: "status", Title: "状态", Width: 96},
	}
	components.Table(c, nil, cols, func() {
		for _, cg := range filtered {
			cg := cg
			spec, _ := labsticker.LookupAnimal(cg.Species)
			components.TableRow(c, cg.ID, cg.CageCode+" "+cg.Strain, false, func(col int) {
				switch col {
				case 0:
					btn := ui.ButtonBase(c.Key("viv-row-" + cg.ID)).Label("查看笼卡 " + cg.CageCode).
						FillWidth().Justify(ui.Start).OnClick(func() {
						w.OpenEditVivariumDialog(cg)
					})
					btn.Children(func() {
						ui.Text(c, cg.CageCode).FontSize(float32(tok.TextData)).Bold().TextColor(tok.Accent).FontFeatures("tnum").SingleLine()
					})
				case 1:
					lbl := cg.Strain
					if spec.Title != "" {
						lbl = spec.Title + " · " + cg.Strain
					}
					ui.Text(c, lbl).FontSize(float32(tok.TextBody)).TextColor(tok.Text).SingleLine().Ellipsis("…")
				case 2:
					ui.Text(c, firstNonEmptyStr(cg.Genotype, "—")).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine().Ellipsis("…")
				case 3:
					ui.Text(c, fmt.Sprintf("%d♂/%d♀", cg.MaleCount, cg.FemaleCount)).FontSize(float32(tok.TextMeta)).FontFeatures("tnum").TextColor(tok.Text).SingleLine()
				case 4:
					ui.Text(c, VivariumAgeLabel(cg.DOB, now)).FontSize(float32(tok.TextMeta)).FontFeatures("tnum").TextColor(tok.TextMuted).SingleLine()
				case 5:
					wt := "—"
					if cg.WeightG > 0 {
						wt = fmt.Sprintf("%.1f", cg.WeightG)
					}
					ui.Text(c, wt).FontSize(float32(tok.TextMeta)).FontFeatures("tnum").TextColor(tok.Text).SingleLine()
				case 6:
					loc := firstNonEmptyStr(strings.TrimSpace(cg.Room+" "+cg.RackPos), vivariumGradeLabel(cg.FacilityGrade))
					ui.Text(c, loc).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine().Ellipsis("…")
				case 7:
					ui.Text(c, firstNonEmptyStr(cg.IACUCNo, "—")).FontSize(float32(tok.TextMeta)).FontFeatures("tnum").TextColor(tok.TextMuted).SingleLine().Ellipsis("…")
				case 8:
					txt, tone := vivariumStatusInfo(cg.Status)
					components.Badge(c, txt, components.BadgeOptions{Tone: tone})
				}
			})
		}
	})
}

func renderVivariumDialog(c *ui.Context, w *Workspace, tok theme.Tokens) {
	if !w.VivariumDialogOpen {
		ui.Box(c.Key("vivarium-dlg")).Children(func() {
			components.Dialog(c, &w.VivariumDialogOpen, components.DialogOptions{Label: "实验动物笼卡"}, nil)
		})
		return
	}
	open := w.VivariumDialogOpen
	t := c.Theme()
	title := "新建动物笼卡"
	if w.vivariumEditingID != "" {
		title = "编辑笼卡 · " + firstNonEmptyStr(w.vivariumDraft.CageCode, w.vivariumDraft.Strain)
	}

	ui.Box(c.Key("vivarium-dlg")).Children(func() {
		components.Dialog(c, &open, components.DialogOptions{Label: title}, func(panel ui.Element) {
			panel.Width(float32(tok.Spacing48)*16.5).MaxHeight(float32(tok.Spacing48)*15.5).Padding(0).Gap(0).Clip()
			kit.ApplyElevation(panel, tok, kit.ElevationModal)
			panel.Children(func() {
				// Top dialog header
				ui.Row(c).FillWidth().Padding(float32(tok.Spacing12), float32(tok.Spacing16)).
					AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					ui.Icon(c, lucide.PawPrint()).Size(float32(tok.IconMd), float32(tok.IconMd)).TextColor(tok.Accent)
					ui.Text(c, title).FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
					ui.Spacer(c)
					if warn := VivariumDensityWarning(VivariumCage{
						Species:     w.vivariumDraft.Species,
						MaleCount:   int(w.vivariumMaleF + 0.5),
						FemaleCount: int(w.vivariumFemaleF + 0.5),
						Status:      w.vivariumDraft.Status,
					}); warn != "" {
						components.Badge(c, "GB 14925 "+warn, components.BadgeOptions{Tone: components.BadgeWarning})
					}
					if components.IconButton(c, lucide.X(), "关闭", components.IconButtonPlain).Clicked() {
						open = false
					}
				})
				ui.Box(c).FillWidth().Height(1).Background(tok.BorderSubtle)

				ui.Scroll(c).FillWidth().Grow(1).MinHeight(0).Children(func() {
					ui.Column(c).FillWidth().Padding(float32(tok.Spacing16)).Gap(float32(tok.Spacing12)).Children(func() {
						if w.vivariumDialogErr != "" {
							kit.InlineBanner(c, tok, lucide.TriangleAlert(), kit.BannerDanger, w.vivariumDialogErr, "", nil)
						}

						// 1. Visual 8-species selector
						ui.Column(c).FillWidth().Gap(float32(tok.Spacing8)).Children(func() {
							ui.Text(c, "实验动物物种载体").FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted)
							ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing8)).Children(func() {
								for _, sp := range labsticker.AllAnimals() {
									sp := sp
									sel := w.vivariumDraft.Species == sp.ID
									tile := ui.ButtonBase(c.Key("viv-sp-" + string(sp.ID))).
										Label(sp.Title).
										Width(float32(tok.Spacing48) * 3.7).
										Padding(float32(tok.Spacing4), float32(tok.Spacing8)).
										Radius(float32(tok.ControlRadius)).
										Background(tok.Elevated).
										Border(1, tok.BorderSubtle)
									if sel {
										tile.Background(tok.AccentSoftBg).Border(1.5, tok.Accent)
									}
									theme.ApplyTactilePress(tile)
									tile.Children(func() {
										ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
											labsticker.RenderAnimalSticker(c, sp.ID, 30)
											ui.Column(c).Grow(1).MinWidth(0).Children(func() {
												ui.Text(c, sp.Title).FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text).SingleLine().Ellipsis("…")
												ui.Text(c, sp.CommonStrains).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine().Ellipsis("…")
											})
										})
									})
									if tile.Clicked() {
										w.vivariumDraft.Species = sp.ID
										if strings.TrimSpace(w.vivariumDraft.FacilityGrade) == "" || w.vivariumEditingID == "" {
											w.vivariumDraft.FacilityGrade = sp.DefaultGrade
										}
										if w.vivariumEditingID == "" {
											w.vivariumDraft.Strain = strings.Split(sp.CommonStrains, " · ")[0]
										}
									}
								}
							})
						})

						// 2. Identification, Strain, Genotype & Census
						row1 := ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing12)).Children(func() {
							ui.Column(c).Width(float32(tok.Spacing48)*3.0).Gap(4).Children(func() {
								ui.Text(c, "笼号 *").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
								components.TextBox(c, &w.vivariumDraft.CageCode, components.TextBoxOptions{}).Placeholder("如 C-01 / IVC-A2")
							})
							ui.Column(c).Grow(1).MinWidth(float32(tok.Spacing48)*4).Gap(4).Children(func() {
								ui.Text(c, "品系 / 背景 *").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
								components.TextBox(c, &w.vivariumDraft.Strain, components.TextBoxOptions{}).Placeholder("C57BL/6J / SD 大鼠 / 新西兰白兔")
							})
							ui.Column(c).Width(float32(tok.Spacing48)*3.4).Gap(4).Children(func() {
								ui.Text(c, "基因型").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
								components.TextBox(c, &w.vivariumDraft.Genotype, components.TextBoxOptions{}).Placeholder("WT / Het / KO")
							})
						})
						_ = row1

						// 3. Sex census & body weight with NumberInput + weight log append
						ui.Row(c).FillWidth().Wrap().AlignItems(ui.End).Gap(float32(tok.Spacing12)).Children(func() {
							ui.Column(c).Width(float32(tok.Spacing48)*3.1).Gap(4).Children(func() {
								ui.Text(c, "雄性 ♂").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
								components.NumberInput(c, &w.vivariumMaleF, 0, 100, 1, components.NumberInputOptions{LeadingText: "♂"}).Label("雄性数量")
							})
							ui.Column(c).Width(float32(tok.Spacing48)*3.1).Gap(4).Children(func() {
								ui.Text(c, "雌性 ♀").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
								components.NumberInput(c, &w.vivariumFemaleF, 0, 100, 1, components.NumberInputOptions{LeadingText: "♀"}).Label("雌性数量")
							})
							ui.Column(c).Width(float32(tok.Spacing48)*3.4).Gap(4).Children(func() {
								ui.Text(c, "当前均重 (g)").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
								components.NumberInput(c, &w.vivariumWeightF, 0, 25000, 0.5, components.NumberInputOptions{LeadingText: "g"}).Label("体重")
							})
							if components.Button(c, "+ 记录今日称重", components.ButtonSecondary).Clicked() {
								today := time.Now().Format("2006-01-02")
								w.vivariumDraft.WeightHistory = append(w.vivariumDraft.WeightHistory, WeightEntry{
									Date:    today,
									WeightG: w.vivariumWeightF,
								})
							}
						})

						if len(w.vivariumDraft.WeightHistory) > 0 {
							ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing4)).AlignItems(ui.Center).Children(func() {
								ui.Text(c, "体重曲线记录：").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
								start := max(0, len(w.vivariumDraft.WeightHistory)-4)
								for _, we := range w.vivariumDraft.WeightHistory[start:] {
									components.Badge(c, fmt.Sprintf("%s: %.1fg", we.Date, we.WeightG), components.BadgeOptions{Tone: components.BadgeNeutral})
								}
							})
						}

						// 4. Housing location, DOB (live age), IACUC & status
						ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing12)).Children(func() {
							ui.Column(c).Width(float32(tok.Spacing48)*3.4).Gap(4).Children(func() {
								ui.Text(c, "出生日期 (DOB)").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
								components.TextBox(c, &w.vivariumDraft.DOB, components.TextBoxOptions{}).Placeholder("YYYY-MM-DD")
								if age := VivariumAgeLabel(w.vivariumDraft.DOB, time.Now()); age != "—" {
									ui.Text(c, "当前："+age).FontSize(float32(tok.TextMeta)).TextColor(tok.Accent).FontFeatures("tnum")
								}
							})
							ui.Column(c).Width(float32(tok.Spacing48)*3.3).Gap(4).Children(func() {
								ui.Text(c, "设施等级").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
								sel := components.Select(c, &w.vivariumDraft.FacilityGrade)
								sel.Trigger.FillWidth().Children(func() {
									components.Text(c, vivariumGradeLabel(w.vivariumDraft.FacilityGrade)).SingleLine()
								})
								sel.Popup(func(ui.Element) {
									for _, g := range []string{"SPF", "conventional", "isolator", "aquatic"} {
										g := g
										sel.Item(g).Children(func() {
											components.Text(c, vivariumGradeLabel(g)).SingleLine()
										})
									}
								})
							})
							ui.Column(c).Width(float32(tok.Spacing48)*2.8).Gap(4).Children(func() {
								ui.Text(c, "房间号").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
								components.TextBox(c, &w.vivariumDraft.Room, components.TextBoxOptions{}).Placeholder("SPF-01 房")
							})
							ui.Column(c).Width(float32(tok.Spacing48)*2.6).Gap(4).Children(func() {
								ui.Text(c, "笼架坐标").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
								components.TextBox(c, &w.vivariumDraft.RackPos, components.TextBoxOptions{}).Placeholder("R2-B4")
							})
						})

						ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing12)).Children(func() {
							ui.Column(c).Width(float32(tok.Spacing48)*3.8).Gap(4).Children(func() {
								ui.Text(c, "伦理批件号 (IACUC)").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
								components.TextBox(c, &w.vivariumDraft.IACUCNo, components.TextBoxOptions{}).Placeholder("如 IACUC-2026-0412")
							})
							ui.Column(c).Width(float32(tok.Spacing48)*3.4).Gap(4).Children(func() {
								ui.Text(c, "个体标记方式与编号").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
								components.TextBox(c, &w.vivariumDraft.AnimalIDs, components.TextBoxOptions{}).Placeholder("剪耳/耳标 #01–#04")
							})
							ui.Column(c).Grow(1).MinWidth(float32(tok.Spacing48)*3.5).Gap(4).Children(func() {
								ui.Text(c, "动物状态").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
								stSel := components.Select(c, &w.vivariumDraft.Status)
								stSel.Trigger.FillWidth().Children(func() {
									lbl, _ := vivariumStatusInfo(w.vivariumDraft.Status)
									components.Text(c, lbl).SingleLine()
								})
								stSel.Popup(func(ui.Element) {
									for _, sCode := range []string{"quarantine", "active", "in_study", "breeding", "archived"} {
										sCode := sCode
										lbl, _ := vivariumStatusInfo(sCode)
										stSel.Item(sCode).Children(func() {
											components.Text(c, lbl).SingleLine()
										})
									}
								})
							})
						})

						ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing12)).Children(func() {
							ui.Column(c).Grow(1).MinWidth(float32(tok.Spacing48)*4).Gap(4).Children(func() {
								ui.Text(c, "供应商 / 合格证号").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
								components.TextBox(c, &w.vivariumDraft.Vendor, components.TextBoxOptions{}).Placeholder("如 维通利华 / SCXK(京)2021-0006")
							})
							ui.Column(c).Grow(1).MinWidth(float32(tok.Spacing48)*4).Gap(4).Children(func() {
								ui.Text(c, "关联实验课题").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
								components.TextBox(c, &w.vivariumDraft.ExperimentRef, components.TextBoxOptions{}).Placeholder("如 HAMA 水凝胶创面愈合实验")
							})
						})

						ui.Column(c).FillWidth().Gap(4).Children(func() {
							ui.Text(c, "动物福利与健康观察（精神/被毛/体重异常/人道终点）").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
							components.TextBox(c, &w.vivariumDraft.HealthNotes, components.TextBoxOptions{Multiline: true}).
								Placeholder("记录采食饮水、手术恢复、肿瘤负荷或丰容环境观察…")
						})
					})
				})

				ui.Box(c).FillWidth().Height(1).Background(tok.BorderSubtle)
				ui.Row(c).FillWidth().Padding(float32(tok.Spacing12), float32(tok.Spacing16)).
					AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
					if w.vivariumEditingID != "" {
						ui.Box(c.Key("viv-del-" + w.vivariumEditingID)).Children(func() {
							h := components.HoldToConfirm(c, &w.vivariumHoldDelReady, "长按删除笼卡", components.HoldToConfirmOptions{
								ConfirmedLabel: "已删除",
								Duration:       900 * time.Millisecond,
								Tone:           components.HoldDanger,
							})
							if h.Changed() {
								w.deleteVivariumCage(w.vivariumEditingID)
								open = false
							}
						})
					}
					ui.Spacer(c)
					if components.Button(c, "取消", components.ButtonGhost).Clicked() {
						open = false
					}
					if components.Button(c, "保存笼卡", components.ButtonPrimary).Clicked() {
						if w.SaveVivariumDraft() {
							open = false
						}
					}
				})
			})
		})
	})
	w.VivariumDialogOpen = open
}
