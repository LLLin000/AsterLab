package tools

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/uikit/labsticker"
	"asterlab-mygo-prototype/modules/runs"
)

// PantryBoxKind distinguishes standard laboratory freezer boxes, EP tube boxes,
// conical tube racks, and 96-well PCR racks that hold positioned reagents.
type PantryBoxKind string

const (
	BoxCryo9x9   PantryBoxKind = "cryobox_9x9"
	BoxCryo10x10 PantryBoxKind = "cryobox_10x10"
	BoxEP50      PantryBoxKind = "tube_box_ep"
	BoxFalcon15  PantryBoxKind = "tube_rack_15ml"
	BoxFalcon50  PantryBoxKind = "tube_rack_50ml"
	BoxPCR96     PantryBoxKind = "pcr_box_96"
)

// PantryBoxSpec describes the geometry, sticker silhouette, and tube defaults of one box preset.
type PantryBoxSpec struct {
	Kind              PantryBoxKind
	Title             string
	ShortTitle        string
	TubeHint          string
	DefaultRows       int
	DefaultCols       int
	DefaultZone       string
	StickerID         labsticker.VesselID
	DefaultTubeVessel labsticker.VesselID
	CellShape         string // "square" | "round" | "pcr"
}

// PantryBox represents a physical freezer box or centrifuge tube rack placed on a pantry tier.
type PantryBox struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Kind     PantryBoxKind `json:"kind"`
	ZoneKey  string        `json:"zoneKey"`            // "-80" | "-20" | "4C" | "ambient"
	ShelfPos string        `json:"shelfPos,omitempty"` // e.g. "二层左"
	Rows     int           `json:"rows"`
	Cols     int           `json:"cols"`
	Notes    string        `json:"notes,omitempty"`
}

var pantryBoxSpecs = []PantryBoxSpec{
	{
		Kind:              BoxCryo9x9,
		Title:             "标准冻存管盒",
		ShortTitle:        "冻存管盒",
		TubeHint:          "适配 1.5 / 2.0 mL 冻存管",
		DefaultRows:       9,
		DefaultCols:       9,
		DefaultZone:       "-80",
		StickerID:         labsticker.VesselBoxCryo9x9,
		DefaultTubeVessel: labsticker.VesselCryovial2,
		CellShape:         "square",
	},
	{
		Kind:              BoxCryo10x10,
		Title:             "小管径冻存盒",
		ShortTitle:        "小管径冻存盒",
		TubeHint:          "适配 0.5 / 1.5 mL 冻存管",
		DefaultRows:       10,
		DefaultCols:       10,
		DefaultZone:       "-80",
		StickerID:         labsticker.VesselBoxCryo9x9,
		DefaultTubeVessel: labsticker.VesselCryovial2,
		CellShape:         "square",
	},
	{
		Kind:              BoxEP50,
		Title:             "EP 离心管盒",
		ShortTitle:        "EP 离心管盒",
		TubeHint:          "适配 1.5 / 2.0 mL EP 管 · 酶管",
		DefaultRows:       5,
		DefaultCols:       10,
		DefaultZone:       "-20",
		StickerID:         labsticker.VesselBoxEP50,
		DefaultTubeVessel: labsticker.VesselEP15,
		CellShape:         "square",
	},
	{
		Kind:              BoxFalcon15,
		Title:             "15 mL 离心管架",
		ShortTitle:        "15 mL 离心管架",
		TubeHint:          "适配 15 mL 锥底离心管",
		DefaultRows:       5,
		DefaultCols:       5,
		DefaultZone:       "4C",
		StickerID:         labsticker.VesselBoxFalcon15,
		DefaultTubeVessel: labsticker.VesselFalcon15,
		CellShape:         "round",
	},
	{
		Kind:              BoxFalcon50,
		Title:             "50 mL 离心管架",
		ShortTitle:        "50 mL 离心管架",
		TubeHint:          "适配 50 mL 大锥底离心管",
		DefaultRows:       4,
		DefaultCols:       4,
		DefaultZone:       "4C",
		StickerID:         labsticker.VesselBoxFalcon50,
		DefaultTubeVessel: labsticker.VesselFalcon50,
		CellShape:         "round",
	},
	{
		Kind:              BoxPCR96,
		Title:             "96 孔 PCR 盒",
		ShortTitle:        "96 孔 PCR 盒",
		TubeHint:          "适配 0.2 mL 单管 / 八联排（固定 8×12）",
		DefaultRows:       8,
		DefaultCols:       12,
		DefaultZone:       "-20",
		StickerID:         labsticker.VesselBoxPCR96,
		DefaultTubeVessel: labsticker.VesselPCRStrip,
		CellShape:         "pcr",
	},
}

// AllPantryBoxSpecs returns the 6 curated storage box and tube rack templates.
func AllPantryBoxSpecs() []PantryBoxSpec {
	out := make([]PantryBoxSpec, len(pantryBoxSpecs))
	copy(out, pantryBoxSpecs)
	return out
}

// LookupPantryBoxSpec returns metadata and default grid size for kind.
func LookupPantryBoxSpec(kind PantryBoxKind) PantryBoxSpec {
	for _, s := range pantryBoxSpecs {
		if s.Kind == kind {
			return s
		}
	}
	return pantryBoxSpecs[0]
}

// FormatBoxSlot converts 0-based (row, col) coordinates into standard plate notation (A1..L12).
func FormatBoxSlot(row, col int) string {
	if row < 0 || row >= 26 || col < 0 {
		return ""
	}
	return fmt.Sprintf("%c%d", 'A'+row, col+1)
}

// NormalizeBoxSlot normalizes inputs like " c02 " -> "C2".
func NormalizeBoxSlot(slot string) string {
	s := strings.ToUpper(strings.TrimSpace(slot))
	if len(s) < 2 {
		return s
	}
	rowChar := s[0]
	if rowChar < 'A' || rowChar > 'Z' {
		return s
	}
	n, err := strconv.Atoi(s[1:])
	if err != nil || n <= 0 {
		return s
	}
	return fmt.Sprintf("%c%d", rowChar, n)
}

// ParseBoxSlot parses "C2" within [0, rows) x [0, cols) to 0-based indices.
func ParseBoxSlot(slot string, rows, cols int) (row, col int, ok bool) {
	s := strings.ToUpper(strings.TrimSpace(slot))
	if len(s) < 2 {
		return 0, 0, false
	}
	r := int(s[0] - 'A')
	c, err := strconv.Atoi(s[1:])
	c--
	if err != nil || r < 0 || c < 0 {
		return 0, 0, false
	}
	if rows > 0 && r >= rows {
		return 0, 0, false
	}
	if cols > 0 && c >= cols {
		return 0, 0, false
	}
	return r, c, true
}

// ItemBoxSlots returns all deduplicated, coordinate-sorted slots occupied by a reagent.
func ItemBoxSlots(meta PantryItemMeta) []string {
	seen := map[string]bool{}
	var slots []string
	add := func(raw string) {
		norm := NormalizeBoxSlot(raw)
		if norm == "" || seen[norm] {
			return
		}
		seen[norm] = true
		slots = append(slots, norm)
	}
	for _, s := range meta.BoxSlots {
		add(s)
	}
	if meta.BoxSlot != "" {
		add(meta.BoxSlot)
	}
	sort.Slice(slots, func(i, j int) bool {
		ri, ci, oki := ParseBoxSlot(slots[i], 26, 99)
		rj, cj, okj := ParseBoxSlot(slots[j], 26, 99)
		if oki && okj {
			if ri != rj {
				return ri < rj
			}
			return ci < cj
		}
		return slots[i] < slots[j]
	})
	return slots
}

// FormatSlotList renders contiguous single-row sequences like "B1–B8" cleanly.
func FormatSlotList(slots []string) string {
	if len(slots) == 0 {
		return ""
	}
	if len(slots) == 1 {
		return slots[0]
	}
	if len(slots) >= 3 {
		r0, c0, ok0 := ParseBoxSlot(slots[0], 26, 99)
		contiguous := ok0
		for i := 1; i < len(slots) && contiguous; i++ {
			ri, ci, oki := ParseBoxSlot(slots[i], 26, 99)
			if !oki || ri != r0 || ci != c0+i {
				contiguous = false
			}
		}
		if contiguous {
			return slots[0] + "–" + slots[len(slots)-1]
		}
	}
	return strings.Join(slots, ", ")
}

// ZoneDisplayPrefix returns a concise human-readable zone prefix for generated locations.
func ZoneDisplayPrefix(zoneKey string) string {
	switch zoneKey {
	case "-80":
		return "-80°C 超低温"
	case "-20":
		return "-20°C 冰箱"
	case "4C":
		return "4°C 冷藏"
	case "ambient":
		return "常温避光柜"
	default:
		return "-20°C 冰箱"
	}
}

// FormatBoxLocation formats a canonical display location string when a reagent lives in a box.
func FormatBoxLocation(zoneKey, boxName, slot string) string {
	parts := make([]string, 0, 3)
	if p := ZoneDisplayPrefix(zoneKey); p != "" {
		parts = append(parts, p)
	}
	if b := strings.TrimSpace(boxName); b != "" {
		parts = append(parts, b)
	}
	if s := strings.TrimSpace(slot); s != "" {
		parts = append(parts, s)
	}
	return strings.Join(parts, " · ")
}

// EnsurePantryBoxesLoaded lazily loads box definitions via Callbacks.LoadPantryBoxes.
func (w *Workspace) EnsurePantryBoxesLoaded() {
	if w.pantryBoxesLoaded {
		return
	}
	w.pantryBoxesLoaded = true
	if w.Callbacks.LoadPantryBoxes == nil {
		return
	}
	if list, err := w.Callbacks.LoadPantryBoxes(); err == nil && list != nil {
		w.PantryBoxes = append([]PantryBox(nil), list...)
	}
}

func (w *Workspace) savePantryBoxes() error {
	if w.Callbacks.SavePantryBoxes != nil {
		return w.Callbacks.SavePantryBoxes(w.PantryBoxes)
	}
	return nil
}

// FindPantryBox returns a box by ID if present.
func (w *Workspace) FindPantryBox(id string) (PantryBox, bool) {
	w.EnsurePantryBoxesLoaded()
	for _, b := range w.PantryBoxes {
		if b.ID == id {
			return b, true
		}
	}
	return PantryBox{}, false
}

// FormatPantryDisplayLocation returns box+slot location when boxed, or item.Location otherwise.
func (w *Workspace) FormatPantryDisplayLocation(item runs.PantryItem) string {
	meta := w.MetaForPantryItem(item)
	if meta.BoxID != "" {
		if box, ok := w.FindPantryBox(meta.BoxID); ok {
			slots := ItemBoxSlots(meta)
			slotDisp := FormatSlotList(slots)
			return FormatBoxLocation(box.ZoneKey, box.Name, slotDisp)
		}
	}
	return item.Location
}

// BoxSlotMap maps each normalized slot coordinate in box to its stored PantryItem.
func (w *Workspace) BoxSlotMap(box PantryBox) map[string]runs.PantryItem {
	out := map[string]runs.PantryItem{}
	for _, it := range w.Pantry.Items {
		meta := w.MetaForPantryItem(it)
		if meta.BoxID != box.ID {
			continue
		}
		for _, sl := range ItemBoxSlots(meta) {
			if _, _, ok := ParseBoxSlot(sl, box.Rows, box.Cols); ok {
				out[sl] = it
			}
		}
	}
	return out
}

// FirstEmptyBoxSlot finds the first unoccupied slot coordinate in box (row-major).
func (w *Workspace) FirstEmptyBoxSlot(box PantryBox) string {
	slotMap := w.BoxSlotMap(box)
	for r := 0; r < box.Rows; r++ {
		for c := 0; c < box.Cols; c++ {
			coord := FormatBoxSlot(r, c)
			if _, taken := slotMap[coord]; !taken {
				return coord
			}
		}
	}
	return ""
}

// BoxStats summarizes occupancy, expiry warnings inside a box, and matching slots for query.
func (w *Workspace) BoxStats(box PantryBox, query string) (occupiedSlots, uniqueItems, nearOrExpired int, matchingSlots []string) {
	q := strings.ToLower(strings.TrimSpace(query))
	seenItem := map[string]bool{}
	seenMatch := map[string]bool{}
	for _, it := range w.Pantry.Items {
		meta := w.MetaForPantryItem(it)
		if meta.BoxID != box.ID {
			continue
		}
		slots := ItemBoxSlots(meta)
		if len(slots) == 0 {
			continue
		}
		if !seenItem[it.ID] {
			seenItem[it.ID] = true
			uniqueItems++
			if it.Status == "near_expiry" || it.Status == "expired" {
				nearOrExpired++
			}
		}
		for _, sl := range slots {
			if _, _, ok := ParseBoxSlot(sl, box.Rows, box.Cols); ok {
				occupiedSlots++
			}
			if q != "" && !seenMatch[sl] {
				hay := strings.ToLower(strings.Join([]string{
					it.Name,
					it.Location,
					pantryItemStatus(it),
					meta.CatalogNo,
					meta.LotNo,
					meta.VolumeNote,
					meta.Notes,
					box.Name,
					box.ShelfPos,
					sl,
				}, " "))
				if strings.Contains(hay, q) {
					seenMatch[sl] = true
					matchingSlots = append(matchingSlots, sl)
				}
			}
		}
	}
	sort.Strings(matchingSlots)
	return occupiedSlots, uniqueItems, nearOrExpired, matchingSlots
}

// OpenPantryBox drills down from the Zone Shelf into a storage box interior view.
func (w *Workspace) OpenPantryBox(boxID, selectSlot string) {
	box, ok := w.FindPantryBox(boxID)
	if !ok {
		return
	}
	w.activeBoxID = boxID
	w.boxMoveItemID = ""
	w.boxMoveFromSlot = ""
	if norm := NormalizeBoxSlot(selectSlot); norm != "" {
		w.selectedBoxSlot = norm
		return
	}
	slotMap := w.BoxSlotMap(box)
	for r := 0; r < box.Rows; r++ {
		for c := 0; c < box.Cols; c++ {
			coord := FormatBoxSlot(r, c)
			if _, occupied := slotMap[coord]; occupied {
				w.selectedBoxSlot = coord
				return
			}
		}
	}
	w.selectedBoxSlot = "A1"
}

// ClosePantryBox returns from a box interior view back up to Level 1 shelves.
func (w *Workspace) ClosePantryBox() {
	w.activeBoxID = ""
	w.selectedBoxSlot = ""
	w.boxMoveItemID = ""
	w.boxMoveFromSlot = ""
}

// LocatePantryItem navigates straight to an item's box+slot (if boxed) or opens its detail dialog.
func (w *Workspace) LocatePantryItem(itemID string) {
	w.Active = ToolPantry
	w.pantrySearch = ""
	w.pantryZoneFilter = "all"
	w.EnsurePantryBoxesLoaded()
	w.EnsurePantryMetaLoaded()
	it, ok := findPantryItemByID(w.Pantry.Items, itemID)
	if !ok {
		return
	}
	meta := w.MetaForPantryItem(it)
	if meta.BoxID != "" {
		if _, hasBox := w.FindPantryBox(meta.BoxID); hasBox {
			slots := ItemBoxSlots(meta)
			sel := "A1"
			if len(slots) > 0 {
				sel = slots[0]
			}
			w.OpenPantryBox(meta.BoxID, sel)
			return
		}
	}
	w.ClosePantryBox()
	w.OpenPantryDetail(it)
}

// OpenNewBoxDialog opens the create-box dialog with defaults for the target zone.
func (w *Workspace) OpenNewBoxDialog(presetZone string) {
	w.EnsurePantryBoxesLoaded()
	kind := BoxCryo9x9
	switch presetZone {
	case "-20":
		kind = BoxEP50
	case "4C":
		kind = BoxFalcon15
	case "ambient":
		kind = BoxEP50
	}
	spec := LookupPantryBoxSpec(kind)
	zone := presetZone
	if zone == "" {
		zone = spec.DefaultZone
	}
	w.boxEditingID = ""
	w.boxDraft = PantryBox{
		Kind:    kind,
		ZoneKey: zone,
		Rows:    spec.DefaultRows,
		Cols:    spec.DefaultCols,
	}
	w.boxDraftRowsStr = strconv.Itoa(spec.DefaultRows)
	w.boxDraftColsStr = strconv.Itoa(spec.DefaultCols)
	w.boxDialogErr = ""
	w.boxConfirmDel = false
	w.PantryBoxDialogOpen = true
}

// OpenEditBoxDialog opens the box configuration dialog for an existing storage box.
func (w *Workspace) OpenEditBoxDialog(box PantryBox) {
	w.EnsurePantryBoxesLoaded()
	w.boxEditingID = box.ID
	w.boxDraft = box
	if w.boxDraft.Rows <= 0 || w.boxDraft.Cols <= 0 {
		sp := LookupPantryBoxSpec(box.Kind)
		w.boxDraft.Rows, w.boxDraft.Cols = sp.DefaultRows, sp.DefaultCols
	}
	w.boxDraftRowsStr = strconv.Itoa(w.boxDraft.Rows)
	w.boxDraftColsStr = strconv.Itoa(w.boxDraft.Cols)
	w.boxDialogErr = ""
	w.boxConfirmDel = false
	w.PantryBoxDialogOpen = true
}

func (w *Workspace) selectBoxDraftPreset(kind PantryBoxKind) {
	spec := LookupPantryBoxSpec(kind)
	w.boxDraft.Kind = kind
	w.boxDraft.Rows = spec.DefaultRows
	w.boxDraft.Cols = spec.DefaultCols
	w.boxDraftRowsStr = strconv.Itoa(spec.DefaultRows)
	w.boxDraftColsStr = strconv.Itoa(spec.DefaultCols)
	if strings.TrimSpace(w.boxDraft.Name) == "" {
		w.boxDraft.Name = spec.Title
	}
}

// SavePantryBoxDialog validates and creates/updates the storage box.
func (w *Workspace) SavePantryBoxDialog() bool {
	w.EnsurePantryBoxesLoaded()
	name := strings.TrimSpace(w.boxDraft.Name)
	if name == "" {
		spec := LookupPantryBoxSpec(w.boxDraft.Kind)
		name = spec.ShortTitle
	}
	rows, errR := strconv.Atoi(strings.TrimSpace(w.boxDraftRowsStr))
	cols, errC := strconv.Atoi(strings.TrimSpace(w.boxDraftColsStr))
	if errR != nil || rows < 2 || rows > 12 {
		w.boxDialogErr = "行数范围需在 2 ~ 12 之间（如 9 或 8）"
		return false
	}
	if errC != nil || cols < 2 || cols > 12 {
		w.boxDialogErr = "列数范围需在 2 ~ 12 之间（如 9 或 12）"
		return false
	}
	zone := w.boxDraft.ZoneKey
	if zone == "" {
		zone = LookupPantryBoxSpec(w.boxDraft.Kind).DefaultZone
	}
	box := PantryBox{
		ID:       w.boxEditingID,
		Name:     name,
		Kind:     w.boxDraft.Kind,
		ZoneKey:  zone,
		ShelfPos: strings.TrimSpace(w.boxDraft.ShelfPos),
		Rows:     rows,
		Cols:     cols,
		Notes:    strings.TrimSpace(w.boxDraft.Notes),
	}
	prevBoxes := append([]PantryBox(nil), w.PantryBoxes...)
	if box.ID == "" {
		box.ID = fmt.Sprintf("box-%d", time.Now().UnixNano()%1000000)
		w.PantryBoxes = append(w.PantryBoxes, box)
	} else {
		updated := false
		for i := range w.PantryBoxes {
			if w.PantryBoxes[i].ID == box.ID {
				w.PantryBoxes[i] = box
				updated = true
				break
			}
		}
		if !updated {
			w.PantryBoxes = append(w.PantryBoxes, box)
		}
	}
	if err := w.savePantryBoxes(); err != nil {
		// Roll back in-memory state on disk failure so no phantom box lingers (UI-11).
		w.PantryBoxes = prevBoxes
		msg := "保存收纳盒失败：" + err.Error()
		if strings.Contains(strings.ToLower(err.Error()), "access is denied") {
			msg += "（本机安全软件可能拦截了未签名程序跨目录写入，请将程序加入白名单或使用正式签名版）"
		}
		w.boxDialogErr = msg
		return false
	}
	w.PantryBoxDialogOpen = false
	w.boxDialogErr = ""
	w.boxConfirmDel = false
	w.Status = "已保存收纳盒：" + box.Name
	return true
}

// DeletePantryBox removes the container and safely unboxes all reagents back onto the zone shelf.
func (w *Workspace) DeletePantryBox(boxID string) error {
	w.EnsurePantryBoxesLoaded()
	w.EnsurePantryMetaLoaded()
	target, ok := w.FindPantryBox(boxID)
	if !ok {
		return nil
	}
	// Safely return any boxed reagents to their zone shelf before removing the box.
	metaChanged := false
	for i := range w.Pantry.Items {
		it := w.Pantry.Items[i]
		meta := w.MetaForPantryItem(it)
		if meta.BoxID == boxID {
			meta.BoxID = ""
			meta.BoxSlot = ""
			meta.BoxSlots = nil
			w.PantryMeta[it.ID] = meta
			metaChanged = true
			fallbackLoc := ZoneDisplayPrefix(target.ZoneKey)
			w.syncPantryLocationRecord(it, fallbackLoc)
		}
	}
	if metaChanged {
		_ = w.savePantryMeta()
	}

	kept := make([]PantryBox, 0, len(w.PantryBoxes))
	for _, b := range w.PantryBoxes {
		if b.ID != boxID {
			kept = append(kept, b)
		}
	}
	w.PantryBoxes = kept
	if err := w.savePantryBoxes(); err != nil {
		return err
	}
	if w.activeBoxID == boxID {
		w.ClosePantryBox()
	}
	w.Status = "已移除收纳盒（盒内试剂已保留在对应温区货架）"
	return nil
}

// PlaceReagentInBoxSlot places or appends an existing reagent into (boxID, slot), guarding collisions.
func (w *Workspace) PlaceReagentInBoxSlot(itemID, boxID, slot string, appendSlot bool) error {
	w.EnsurePantryBoxesLoaded()
	w.EnsurePantryMetaLoaded()
	box, ok := w.FindPantryBox(boxID)
	if !ok {
		return fmt.Errorf("未找到目标收纳盒")
	}
	norm := NormalizeBoxSlot(slot)
	if _, _, valid := ParseBoxSlot(norm, box.Rows, box.Cols); !valid {
		return fmt.Errorf("孔位坐标 %q 超出盒子范围（最大 %c%d）", slot, 'A'+box.Rows-1, box.Cols)
	}
	item, ok := findPantryItemByID(w.Pantry.Items, itemID)
	if !ok {
		return fmt.Errorf("未找到试剂记录")
	}
	slotMap := w.BoxSlotMap(box)
	if existing, occupied := slotMap[norm]; occupied && existing.ID != itemID {
		return fmt.Errorf("孔位 %s 已被「%s」占用", norm, existing.Name)
	}

	meta := w.MetaForPantryItem(item)
	var nextSlots []string
	if appendSlot && meta.BoxID == boxID {
		nextSlots = append(nextSlots, ItemBoxSlots(meta)...)
	}
	found := false
	for _, s := range nextSlots {
		if s == norm {
			found = true
			break
		}
	}
	if !found {
		nextSlots = append(nextSlots, norm)
	}
	meta.BoxID = boxID
	meta.BoxSlot = nextSlots[0]
	if len(nextSlots) > 1 {
		meta.BoxSlots = nextSlots
	} else {
		meta.BoxSlots = nil
	}
	w.PantryMeta[itemID] = meta
	if err := w.savePantryMeta(); err != nil {
		return err
	}
	w.syncPantryLocationRecord(item, FormatBoxLocation(box.ZoneKey, box.Name, FormatSlotList(nextSlots)))
	w.selectedBoxSlot = norm
	w.Error = ""
	w.Status = fmt.Sprintf("已将「%s」放入 %s · %s", item.Name, box.Name, norm)
	return nil
}

// PlaceReagentInPCRStrip assigns an entire 8-well row (columns 1..8) of a PCR box to itemID.
func (w *Workspace) PlaceReagentInPCRStrip(itemID, boxID string, rowIdx int) error {
	box, ok := w.FindPantryBox(boxID)
	if !ok {
		return fmt.Errorf("未找到 PCR 盒")
	}
	if rowIdx < 0 || rowIdx >= box.Rows || box.Cols < 8 {
		return fmt.Errorf("当前行不支持完整 8 联排占位")
	}
	item, ok := findPantryItemByID(w.Pantry.Items, itemID)
	if !ok {
		return fmt.Errorf("未找到试剂记录")
	}
	slotMap := w.BoxSlotMap(box)
	stripSlots := make([]string, 0, 8)
	for col := 0; col < 8; col++ {
		coord := FormatBoxSlot(rowIdx, col)
		if existing, occupied := slotMap[coord]; occupied && existing.ID != itemID {
			return fmt.Errorf("该行 %s 已被「%s」占用，无法整条放置八联排", coord, existing.Name)
		}
		stripSlots = append(stripSlots, coord)
	}
	meta := w.MetaForPantryItem(item)
	meta.BoxID = boxID
	meta.BoxSlot = stripSlots[0]
	meta.BoxSlots = stripSlots
	w.PantryMeta[itemID] = meta
	if err := w.savePantryMeta(); err != nil {
		return err
	}
	w.syncPantryLocationRecord(item, FormatBoxLocation(box.ZoneKey, box.Name, FormatSlotList(stripSlots)))
	w.selectedBoxSlot = stripSlots[0]
	w.Status = fmt.Sprintf("已整条放置八联排「%s」于 %s %s", item.Name, box.Name, FormatSlotList(stripSlots))
	return nil
}

// MoveReagentSlot moves an item from fromSlot to an empty toSlot inside boxID.
func (w *Workspace) MoveReagentSlot(itemID, boxID, fromSlot, toSlot string) error {
	box, ok := w.FindPantryBox(boxID)
	if !ok {
		return fmt.Errorf("未找到收纳盒")
	}
	fromNorm := NormalizeBoxSlot(fromSlot)
	toNorm := NormalizeBoxSlot(toSlot)
	if _, _, valid := ParseBoxSlot(toNorm, box.Rows, box.Cols); !valid {
		return fmt.Errorf("目标孔位 %q 超出盒子范围", toSlot)
	}
	slotMap := w.BoxSlotMap(box)
	if existing, taken := slotMap[toNorm]; taken {
		return fmt.Errorf("目标孔位 %s 已被「%s」占用", toNorm, existing.Name)
	}
	item, ok := findPantryItemByID(w.Pantry.Items, itemID)
	if !ok {
		return fmt.Errorf("未找到试剂记录")
	}
	meta := w.MetaForPantryItem(item)
	curSlots := ItemBoxSlots(meta)
	if len(curSlots) <= 1 {
		meta.BoxID = boxID
		meta.BoxSlot = toNorm
		meta.BoxSlots = nil
	} else {
		next := make([]string, 0, len(curSlots))
		replaced := false
		for _, s := range curSlots {
			if s == fromNorm {
				next = append(next, toNorm)
				replaced = true
			} else {
				next = append(next, s)
			}
		}
		if !replaced {
			next = append(next, toNorm)
		}
		meta.BoxID = boxID
		meta.BoxSlot = ""
		meta.BoxSlots = next
		normSlots := ItemBoxSlots(meta)
		meta.BoxSlot = normSlots[0]
		meta.BoxSlots = normSlots
	}
	w.PantryMeta[itemID] = meta
	if err := w.savePantryMeta(); err != nil {
		return err
	}
	w.syncPantryLocationRecord(item, FormatBoxLocation(box.ZoneKey, box.Name, FormatSlotList(ItemBoxSlots(meta))))
	w.selectedBoxSlot = toNorm
	w.boxMoveItemID = ""
	w.boxMoveFromSlot = ""
	w.Status = fmt.Sprintf("已将「%s」移至 %s · %s", item.Name, box.Name, toNorm)
	return nil
}

// RemoveSlotFromReagent frees a single coordinate (or unboxes the item if it only held one slot).
func (w *Workspace) RemoveSlotFromReagent(itemID, slot string) error {
	item, ok := findPantryItemByID(w.Pantry.Items, itemID)
	if !ok {
		return nil
	}
	meta := w.MetaForPantryItem(item)
	slots := ItemBoxSlots(meta)
	norm := NormalizeBoxSlot(slot)
	kept := make([]string, 0, len(slots))
	for _, s := range slots {
		if s != norm {
			kept = append(kept, s)
		}
	}
	if len(kept) == 0 {
		return w.RemoveReagentFromBox(itemID)
	}
	box, hasBox := w.FindPantryBox(meta.BoxID)
	meta.BoxSlot = kept[0]
	if len(kept) > 1 {
		meta.BoxSlots = kept
	} else {
		meta.BoxSlots = nil
	}
	w.PantryMeta[itemID] = meta
	if err := w.savePantryMeta(); err != nil {
		return err
	}
	if hasBox {
		w.syncPantryLocationRecord(item, FormatBoxLocation(box.ZoneKey, box.Name, FormatSlotList(kept)))
	}
	w.Status = fmt.Sprintf("已释放孔位 %s", norm)
	return nil
}

// RemoveReagentFromBox detaches a reagent from its box and leaves it on its parent temperature shelf.
func (w *Workspace) RemoveReagentFromBox(itemID string) error {
	item, ok := findPantryItemByID(w.Pantry.Items, itemID)
	if !ok {
		return nil
	}
	meta := w.MetaForPantryItem(item)
	zoneFallback := item.Location
	if box, hasBox := w.FindPantryBox(meta.BoxID); hasBox {
		zoneFallback = ZoneDisplayPrefix(box.ZoneKey)
	}
	meta.BoxID = ""
	meta.BoxSlot = ""
	meta.BoxSlots = nil
	w.PantryMeta[itemID] = meta
	if err := w.savePantryMeta(); err != nil {
		return err
	}
	w.syncPantryLocationRecord(item, zoneFallback)
	w.Status = fmt.Sprintf("已将「%s」移出收纳盒，保留在货架", item.Name)
	return nil
}

func (w *Workspace) syncPantryLocationRecord(item runs.PantryItem, loc string) {
	for i := range w.Pantry.Items {
		if w.Pantry.Items[i].ID == item.ID {
			w.Pantry.Items[i].Location = loc
			break
		}
	}
	if w.Callbacks.UpsertPantry == nil {
		return
	}
	rev := item.Revision
	updated, err := w.Callbacks.UpsertPantry(runs.PantryUpsert{
		ID:                  item.ID,
		Name:                item.Name,
		Location:            loc,
		PurchasedOn:         item.PurchasedOn,
		OpenedOn:            item.OpenedOn,
		ShelfLifeDays:       item.ShelfLifeDays,
		OpenedShelfLifeDays: item.OpenedShelfLifeDays,
		Exhausted:           item.Exhausted,
		ExpectedRevision:    &rev,
	})
	if err == nil {
		for i := range w.Pantry.Items {
			if w.Pantry.Items[i].ID == updated.ID {
				w.Pantry.Items[i] = updated
				break
			}
		}
	}
}

// OpenNewPantryDialogInBox opens the new reagent creation dialog pre-bound to (box, slot).
func (w *Workspace) OpenNewPantryDialogInBox(box PantryBox, slot string) {
	spec := LookupPantryBoxSpec(box.Kind)
	norm := NormalizeBoxSlot(slot)
	if norm == "" {
		norm = w.FirstEmptyBoxSlot(box)
	}
	w.clearPantryForm()
	w.PantryLocation = FormatBoxLocation(box.ZoneKey, box.Name, norm)
	itemType := "reagent"
	sampleKind := ""
	if box.Kind == BoxCryo9x9 || box.Kind == BoxCryo10x10 || box.ZoneKey == "-80" {
		// UI-12: Cryo boxes primarily store frozen cells, bacterial strains, plasmids, or tissue samples.
		itemType = "biosample"
		sampleKind = "细胞株"
	}
	w.pantryDraftMeta = PantryItemMeta{
		ItemType:   itemType,
		SampleKind: sampleKind,
		FrozenOn:   time.Now().Format("2006-01-02"),
		VesselID:   spec.DefaultTubeVessel,
		BoxID:      box.ID,
		BoxSlot:    norm,
	}
	w.pantryStatusIdx = 0
	w.pantryPickerOpen = false
	w.pantryConfirmDel = false
	w.pantryDialogErr = ""
	w.PantryDialogOpen = true
}

func boxSlotSearchMatches(query string, box PantryBox, slotMap map[string]runs.PantryItem, w *Workspace) map[string]bool {
	out := map[string]bool{}
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return out
	}
	for coord, it := range slotMap {
		meta := w.MetaForPantryItem(it)
		hay := strings.ToLower(strings.Join([]string{
			coord,
			it.Name,
			pantryItemStatus(it),
			meta.CatalogNo,
			meta.LotNo,
			meta.VolumeNote,
			meta.Notes,
			box.Name,
		}, " "))
		if strings.Contains(hay, q) {
			out[coord] = true
		}
	}
	return out
}

func renderPantryBoxInteriorView(c *ui.Context, w *Workspace, tok theme.Tokens, box PantryBox) {
	t := c.Theme()
	spec := LookupPantryBoxSpec(box.Kind)
	slotMap := w.BoxSlotMap(box)
	occupied, uniqueCnt, alertCnt, _ := w.BoxStats(box, "")
	totalSlots := box.Rows * box.Cols
	if w.selectedBoxSlot == "" {
		w.selectedBoxSlot = "A1"
	} else if _, _, ok := ParseBoxSlot(w.selectedBoxSlot, box.Rows, box.Cols); !ok {
		w.selectedBoxSlot = "A1"
	}

	// 1. Breadcrumb Header
	ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
		if components.Button(c, "← 返回货架", components.ButtonSecondary).Clicked() {
			w.ClosePantryBox()
			return
		}
		ui.Row(c).Grow(1).MinWidth(0).AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
			ui.Text(c, ZoneDisplayPrefix(box.ZoneKey)+" /").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).Shrink(0)
			ui.Text(c, box.Name).FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text).
				SingleLine().Ellipsis("…").MinWidth(0)
			components.Badge(c, fmt.Sprintf("%s · %d×%d", spec.ShortTitle, box.Rows, box.Cols), components.BadgeOptions{Tone: components.BadgeNeutral})
			occTone := components.BadgeAccent
			if alertCnt > 0 {
				occTone = components.BadgeWarning
			}
			components.Badge(c, fmt.Sprintf("已放 %d 管 / %d/%d 孔", uniqueCnt, occupied, totalSlots), components.BadgeOptions{Tone: occTone})
		})
		if components.Button(c, "编辑盒子", components.ButtonGhost).Clicked() {
			w.OpenEditBoxDialog(box)
		}
		if components.Button(c, "+ 在空位新增试剂", components.ButtonPrimary).Clicked() {
			targetSlot := w.selectedBoxSlot
			if _, taken := slotMap[targetSlot]; taken {
				targetSlot = w.FirstEmptyBoxSlot(box)
			}
			w.OpenNewPantryDialogInBox(box, targetSlot)
		}
	})

	// 2. Quick Box Switcher Strip
	if len(w.PantryBoxes) > 1 {
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
			ui.Text(c, "切换收纳盒：").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
			for _, bx := range w.PantryBoxes {
				bx := bx
				active := bx.ID == box.ID
				variant := components.ButtonGhost
				if active {
					variant = components.ButtonToolbar
				}
				if components.Button(c, bx.Name, variant).Clicked() && !active {
					w.OpenPantryBox(bx.ID, "")
				}
			}
		})
	}

	// 3. Inline Search + Move Banner
	ui.Row(c).FillWidth().Wrap().Padding(t.Space(2), t.Space(3)).
		Radius(float32(tok.PanelRadius)).Background(tok.Elevated).Border(1, tok.Border).
		AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
		ui.Box(c).Width(float32(tok.Spacing48) * 5.6).Children(func() {
			components.TextBox(c, &w.pantrySearch, components.TextBoxOptions{
				LeadingIcon: lucide.Search(),
			}).Placeholder("搜盒内试剂、货号、批号或孔位…")
		})
		if strings.TrimSpace(w.pantrySearch) != "" {
			if components.Button(c, "清除高亮", components.ButtonGhost).Clicked() {
				w.pantrySearch = ""
			}
		}
		if w.boxMoveItemID != "" {
			moveLabel := "请选择空孔位"
			if it, ok := findPantryItemByID(w.Pantry.Items, w.boxMoveItemID); ok {
				moveLabel = fmt.Sprintf("移动模式：请点击左侧空孔位将「%s」从 %s 移入", it.Name, w.boxMoveFromSlot)
			}
			components.Badge(c, moveLabel, components.BadgeOptions{Tone: components.BadgeAccent})
			if components.Button(c, "取消移动", components.ButtonSecondary).Clicked() {
				w.boxMoveItemID = ""
				w.boxMoveFromSlot = ""
			}
		} else {
			ui.Spacer(c)
			ui.Text(c, spec.TubeHint).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
		}
	})

	// 4. Two-Column Spatial Grid (Left) + Inspector & Sorted Manifest (Right)
	ui.Row(c).FillWidth().Gap(t.Space(4)).AlignItems(ui.Start).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(t.Space(2)).Children(func() {
			renderBoxSlotGrid(c, w, tok, box, spec, slotMap)
		})
		ui.Column(c).Width(float32(tok.Spacing48)*6.4).Shrink(0).Gap(t.Space(3)).Children(func() {
			renderBoxInspectorPanel(c, w, tok, box, spec, slotMap)
		})
	})
}

func renderBoxSlotGrid(
	c *ui.Context,
	w *Workspace,
	tok theme.Tokens,
	box PantryBox,
	spec PantryBoxSpec,
	slotMap map[string]runs.PantryItem,
) {
	t := c.Theme()
	dark := t.Dark
	matches := boxSlotSearchMatches(w.pantrySearch, box, slotMap, w)
	hasQuery := strings.TrimSpace(w.pantrySearch) != ""

	var cellMinH float32
	switch {
	case box.Cols <= 4:
		cellMinH = float32(tok.Spacing48) * 1.85
	case box.Cols <= 5:
		cellMinH = float32(tok.Spacing48) * 1.5
	case box.Cols <= 9:
		cellMinH = float32(tok.Spacing48) * 1.05
	case box.Cols <= 10:
		cellMinH = float32(tok.Spacing48) * 0.96
	default:
		cellMinH = float32(tok.Spacing48) * 0.84
	}

	cellRadius := float32(tok.ControlRadius)
	if spec.CellShape == "round" || spec.CellShape == "pcr" {
		cellRadius = cellMinH * 0.5
	}

	rowAxisW := float32(tok.Spacing24)

	ui.Column(c).FillWidth().Padding(t.Space(3)).Radius(float32(tok.PanelRadius)).
		Background(tok.Elevated).Border(1, tok.BorderStrong).
		InsetShadow(0, 1, 3, 0, tok.Text.Alpha(0.05)).
		Gap(t.Space(1)).Children(func() {
		// Column numbers header (1..Cols)
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
			ui.Box(c).Width(rowAxisW)
			for col := 0; col < box.Cols; col++ {
				ui.Box(c).Grow(1).MinWidth(0).Center().Children(func() {
					ui.Text(c, strconv.Itoa(col+1)).FontSize(float32(tok.TextMeta)).
						FontFeatures("tnum").Bold().TextColor(tok.TextMuted)
				})
			}
		})

		// Spatial grid rows (A..rows)
		for row := 0; row < box.Rows; row++ {
			rowLabel := fmt.Sprintf("%c", 'A'+row)
			ui.Row(c).FillWidth().AlignItems(ui.Stretch).Gap(t.Space(1)).Children(func() {
				ui.Box(c).Width(rowAxisW).Center().Children(func() {
					ui.Text(c, rowLabel).FontSize(float32(tok.TextMeta)).
						FontFeatures("tnum").Bold().TextColor(tok.TextMuted)
				})
				for col := 0; col < box.Cols; col++ {
					coord := FormatBoxSlot(row, col)
					item, occupied := slotMap[coord]
					selected := coord == w.selectedBoxSlot
					isMatch := matches[coord]

					bg := tok.Surface.Alpha(0.75)
					border := tok.Border
					var borderW float32 = 1

					if occupied {
						capCol := theme.CategoricalColor(item.ID, dark)
						if item.Status == "exhausted" {
							bg = tok.Surface
						} else {
							bg = capCol.Alpha(0.14)
							if dark {
								bg = capCol.Alpha(0.20)
							}
						}
						border = capCol.Alpha(0.45)
						if hasQuery && !isMatch {
							bg = tok.Surface.Alpha(0.45)
							border = tok.Border
						}
					}
					if isMatch {
						bg = tok.AccentSoftBg
						border = tok.Accent
						borderW = 2
					}
					if selected {
						bg = tok.AccentSoftBg
						border = tok.Accent
						borderW = 2
					}

					cellBtn := ui.ButtonBase(c.Key("box-slot-"+box.ID+"-"+coord)).
						Label("孔位 "+coord).
						Grow(1).Basis(0).MinWidth(0).MinHeight(cellMinH).
						Padding(t.Space(1), t.Space(0)).
						Radius(cellRadius).
						Center().
						OnClick(func() {
							if w.boxMoveItemID != "" && !occupied {
								if err := w.MoveReagentSlot(w.boxMoveItemID, box.ID, w.boxMoveFromSlot, coord); err != nil {
									w.Error = err.Error()
								}
								return
							}
							w.selectedBoxSlot = coord
						})

					targetBorder := border
					targetBg := bg
					if cellBtn.Hovered() && !selected {
						targetBorder = tok.Accent.Alpha(0.75)
						targetBg = bg.Mix(tok.Accent, 0.04)
					}
					cellBtn.Background(cellBtn.AnimateColor("cell-bg", targetBg, theme.MotionSnappy)).
						Border(borderW, cellBtn.AnimateColor("cell-bdr", targetBorder, theme.MotionSnappy))

					cellBtn.Children(func() {
						renderBoxCellContent(c, tok, box, spec, coord, item, occupied, selected, isMatch, dark)
					})

					if cellBtn.DoubleClicked() {
						// UI-12: Double-clicking an empty slot immediately creates a new reagent in that slot;
						// double-clicking an occupied slot opens its detail dialog.
						w.selectedBoxSlot = coord
						if occupied {
							w.OpenPantryDetail(item)
						} else {
							w.OpenNewPantryDialogInBox(box, coord)
						}
					}

					if occupied {
						tip := fmt.Sprintf("%s · %s · %s · 双击查看详情", coord, item.Name, pantryItemStatus(item))
						if m, ok := w.PantryMeta[item.ID]; ok && m.LotNo != "" {
							tip += " · Lot " + m.LotNo
						}
						components.Tooltip(c, cellBtn, tip)
					} else {
						tip := coord + " · 空位 · 双击新建试剂"
						if w.boxMoveItemID != "" {
							tip = "点击将试剂移入 " + coord
						}
						components.Tooltip(c, cellBtn, tip)
					}
				}
			})
		}

		// Compact bottom legend
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(3)).
			Padding(t.Space(2), t.Space(1), t.Space(0), t.Space(1)).Children(func() {
			ui.Text(c, "图例：").FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted)
			ui.Row(c).AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
				dotSz := float32(tok.Spacing12)
				ui.Box(c).Size(dotSz, dotSz).Radius(dotSz * 0.5).Background(tok.Accent)
				ui.Text(c, "在库试剂管（按试剂配色）").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
			})
			ui.Row(c).AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
				dotSz := float32(tok.Spacing12)
				ui.Box(c).Size(dotSz, dotSz).Radius(dotSz * 0.5).Background(tok.Warning)
				ui.Text(c, "临期/过期警戒").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
			})
			ui.Row(c).AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
				dotSz := float32(tok.Spacing12)
				ui.Box(c).Size(dotSz, dotSz).Radius(dotSz * 0.5).Background(tok.Surface).Border(1, tok.BorderStrong)
				ui.Text(c, "空孔位（点选后在右侧快捷放入）").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
			})
		})
	})
}

func renderBoxCellContent(
	c *ui.Context,
	tok theme.Tokens,
	box PantryBox,
	spec PantryBoxSpec,
	coord string,
	item runs.PantryItem,
	occupied, selected, isMatch, dark bool,
) {
	t := c.Theme()
	if !occupied {
		coordColor := tok.TextMuted.Alpha(0.75)
		if selected {
			coordColor = tok.Accent
		}
		ui.Column(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(0)).Children(func() {
			ui.Text(c, coord).FontSize(float32(tok.TextMeta)).FontFeatures("tnum").TextColor(coordColor)
		})
		return
	}

	capCol := theme.CategoricalColor(item.ID, dark)
	if item.Status == "exhausted" {
		capCol = tok.TextMuted.Alpha(0.55)
	}
	ringColor := tok.Surface
	if item.Status == "near_expiry" {
		ringColor = tok.Warning
	} else if item.Status == "expired" {
		ringColor = tok.Danger
	} else if isMatch || selected {
		ringColor = tok.Accent
	}

	var capSz float32
	switch {
	case box.Cols <= 5:
		capSz = float32(tok.Spacing24)
	case box.Cols <= 10:
		capSz = float32(tok.Spacing16)
	default:
		capSz = float32(tok.Spacing12)
	}

	// Render adaptive content density so 4x4 racks show full names while 9x9/8x12 stay crisp
	ui.Column(c).FillWidth().Padding(t.Space(0), t.Space(1)).AlignItems(ui.Center).Gap(t.Space(0)).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.Center).Gap(t.Space(1)).Children(func() {
			ui.Text(c, coord).FontSize(float32(tok.TextMeta)).FontFeatures("tnum").Bold().TextColor(tok.TextMuted)
			ui.Box(c).Size(capSz, capSz).Radius(capSz * 0.5).
				Background(capCol).Border(t.Space(0)+1, ringColor)
		})
		if box.Cols <= 5 {
			ui.Text(c, item.Name).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Text).
				SingleLine().Ellipsis("…").MinWidth(0)
			if box.Cols <= 4 {
				ui.Text(c, pantryItemStatus(item)).FontSize(float32(tok.TextMeta)).TextColor(boxStatusTextColor(tok, item.Status))
			}
		} else if box.Cols <= 10 {
			shortLabel := truncateRunes(item.Name, 5)
			ui.Text(c, shortLabel).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Text).
				SingleLine().Ellipsis("…").MinWidth(0)
		} else {
			// Compact 96-well PCR grid: clear single initial or strip badge + cap dot above
			ui.Text(c, truncateRunes(item.Name, 3)).FontSize(float32(tok.TextMeta)).TextColor(tok.Text).
				SingleLine().MinWidth(0)
		}
	})
}

func renderBoxInspectorPanel(
	c *ui.Context,
	w *Workspace,
	tok theme.Tokens,
	box PantryBox,
	spec PantryBoxSpec,
	slotMap map[string]runs.PantryItem,
) {
	t := c.Theme()
	selSlot := w.selectedBoxSlot
	item, occupied := slotMap[selSlot]

	// Card A: Selected Coordinate Inspector
	ui.Column(c).FillWidth().Padding(t.Space(3)).Radius(float32(tok.PanelRadius)).
		Background(tok.Surface).Border(1, tok.Border).Gap(t.Space(2)).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
			components.Badge(c, "孔位 "+selSlot, components.BadgeOptions{Tone: components.BadgeAccent})
			if occupied {
				components.Badge(c, pantryItemStatus(item), components.BadgeOptions{Tone: detailStatusTone(item.Status)})
			} else {
				components.Badge(c, "空孔位", components.BadgeOptions{Tone: components.BadgeNeutral})
			}
			ui.Spacer(c)
			if occupied {
				ui.Icon(c, lucide.FlaskConical()).Size(t.Space(4), t.Space(4)).TextColor(tok.Accent)
			}
		})

		if occupied {
			meta := w.MetaForPantryItem(item)
			allSlots := ItemBoxSlots(meta)
			ui.Row(c).FillWidth().AlignItems(ui.Start).Gap(t.Space(2)).Children(func() {
				ui.Box(c).Padding(t.Space(1)).Radius(float32(tok.ControlRadius)).
					Background(tok.Elevated).Border(1, tok.Border).Children(func() {
					labsticker.RenderSticker(c, meta.VesselID, w.loadCustomStickerImage(meta.CustomStickerRel), labsticker.StickerOptions{
						TargetHeightDIP: 52,
						BorderDIP:       3.5,
						Exhausted:       item.Status == "exhausted",
					})
				})
				ui.Column(c).Grow(1).MinWidth(0).Gap(t.Space(0)).Children(func() {
					ui.Text(c, item.Name).FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text)
					if meta.CatalogNo != "" || meta.LotNo != "" {
						ui.Text(c, strings.Trim(meta.CatalogNo+" · Lot "+meta.LotNo, " ·Lot")).
							FontSize(float32(tok.TextMeta)).FontFeatures("tnum").TextColor(tok.TextMuted)
					}
					if item.ExpiresOn != "" {
						ui.Text(c, "效期 "+item.ExpiresOn).FontSize(float32(tok.TextMeta)).
							FontFeatures("tnum").TextColor(tok.TextMuted)
					}
					if len(allSlots) > 1 {
						ui.Text(c, "占用孔位："+FormatSlotList(allSlots)).FontSize(float32(tok.TextMeta)).
							FontFeatures("tnum").Bold().TextColor(tok.Accent)
					}
					var linkNotes []string
					if len(meta.Manuals) > 0 {
						linkNotes = append(linkNotes, fmt.Sprintf("附 %d 份说明书", len(meta.Manuals)))
					}
					if strings.TrimSpace(meta.ProductURL) != "" {
						linkNotes = append(linkNotes, "官网已绑定")
					}
					if len(linkNotes) > 0 {
						ui.Text(c, strings.Join(linkNotes, " · ")).
							FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
					}
				})
			})

			ui.Row(c).FillWidth().Wrap().Gap(t.Space(2)).Children(func() {
				if components.Button(c, "查看档案与说明书", components.ButtonPrimary).Clicked() {
					w.OpenPantryDetail(item)
				}
				if components.Button(c, "移至其他孔位", components.ButtonSecondary).Clicked() {
					w.boxMoveItemID = item.ID
					w.boxMoveFromSlot = selSlot
				}
				if len(allSlots) > 1 {
					if components.Button(c, "释放此孔", components.ButtonGhost).Clicked() {
						_ = w.RemoveSlotFromReagent(item.ID, selSlot)
					}
				} else {
					if components.Button(c, "移出本盒", components.ButtonGhost).Clicked() {
						_ = w.RemoveReagentFromBox(item.ID)
					}
				}
			})
		} else {
			ui.Text(c, fmt.Sprintf("当前选中空孔位 %s，可以直接新增管/试剂，或从货架未入盒试剂快速放进来。", selSlot)).
				FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)

			ui.Row(c).FillWidth().Wrap().Gap(t.Space(2)).Children(func() {
				if components.Button(c, "+ 在 "+selSlot+" 新建试剂", components.ButtonPrimary).Clicked() {
					w.OpenNewPantryDialogInBox(box, selSlot)
				}
				if box.Kind == BoxPCR96 {
					rIdx, _, valid := ParseBoxSlot(selSlot, box.Rows, box.Cols)
					if valid && box.Cols >= 8 {
						// Quick helper if there's a loose PCR strip or any loose reagent in -20°C
						if loose := w.looseReagentsForBox(box); len(loose) > 0 {
							firstLoose := loose[0]
							if components.Button(c, fmt.Sprintf("整排放入 %s", truncateRunes(firstLoose.Name, 6)), components.ButtonSecondary).Clicked() {
								if err := w.PlaceReagentInPCRStrip(firstLoose.ID, box.ID, rIdx); err != nil {
									w.Error = err.Error()
								}
							}
						}
					}
				}
			})

			loose := w.looseReagentsForBox(box)
			if len(loose) > 0 {
				ui.Divider(c)
				ui.Text(c, "从货架快速放入 "+selSlot+"：").FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Text)
				limit := min(5, len(loose))
				for _, cand := range loose[:limit] {
					cand := cand
					ui.Row(c.Key("loose-"+cand.ID)).FillWidth().Padding(t.Space(1), t.Space(2)).
						Radius(float32(tok.ControlRadius)).Background(tok.Elevated).Border(1, tok.Border).
						AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
						ui.Text(c, cand.Name).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Text).
							Grow(1).MinWidth(0).SingleLine().Ellipsis("…")
						if components.Button(c, "放入", components.ButtonGhost).Clicked() {
							if err := w.PlaceReagentInBoxSlot(cand.ID, box.ID, selSlot, false); err != nil {
								w.Error = err.Error()
							}
						}
					})
				}
			}
		}
	})

	// Card B: Sorted Box Manifest
	type manifestEntry struct {
		slot string
		item runs.PantryItem
	}
	var entries []manifestEntry
	seen := map[string]bool{}
	for r := 0; r < box.Rows; r++ {
		for col := 0; col < box.Cols; col++ {
			coord := FormatBoxSlot(r, col)
			it, ok := slotMap[coord]
			if !ok || seen[it.ID] {
				continue
			}
			seen[it.ID] = true
			entries = append(entries, manifestEntry{slot: coord, item: it})
		}
	}

	ui.Column(c).FillWidth().Padding(t.Space(3)).Radius(float32(tok.PanelRadius)).
		Background(tok.Surface).Border(1, tok.Border).Gap(t.Space(2)).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
			ui.Text(c, "盒内试剂清单").FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text).Grow(1).MinWidth(0)
			components.Badge(c, fmt.Sprintf("%d 项", len(entries)), components.BadgeOptions{Tone: components.BadgeNeutral})
		})
		if len(entries) == 0 {
			ui.Text(c, "盒内暂无已摆放的试剂，点击左侧任意空孔位放入。").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
			return
		}
		ui.Scroll(c).FillWidth().MaxHeight(float32(tok.Spacing48)*5.4).Children(func() {
			ui.Column(c).FillWidth().Gap(t.Space(1)).Children(func() {
				for _, e := range entries {
					e := e
					meta := w.MetaForPantryItem(e.item)
					slotsDisp := FormatSlotList(ItemBoxSlots(meta))
					isCur := e.slot == selSlot || (occupied && item.ID == e.item.ID)
					rowBg := tok.Elevated
					rowBorder := tok.Border
					if isCur {
						rowBg = tok.AccentSoftBg
						rowBorder = tok.Accent
					}
					btn := ui.ButtonBase(c.Key("manifest-"+e.item.ID)).Label("定位 "+e.item.Name).
						FillWidth().Padding(t.Space(1), t.Space(2)).
						Radius(float32(tok.ControlRadius)).Background(rowBg).Border(1, rowBorder).
						OnClick(func() {
							w.selectedBoxSlot = e.slot
						})
					btn.Children(func() {
						ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
							components.Badge(c, slotsDisp, components.BadgeOptions{Tone: components.BadgeNeutral})
							dotSz := float32(tok.Spacing8)
							ui.Box(c).Size(dotSz, dotSz).Radius(dotSz * 0.5).
								Background(theme.CategoricalColor(e.item.ID, t.Dark)).Shrink(0)
							ui.Text(c, e.item.Name).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Text).
								Grow(1).MinWidth(0).SingleLine().Ellipsis("…")
							ui.Text(c, pantryItemStatus(e.item)).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).Shrink(0)
						})
					})
				}
			})
		})
	})
}

func (w *Workspace) looseReagentsForBox(box PantryBox) []runs.PantryItem {
	var matchingZone, other []runs.PantryItem
	for _, it := range w.Pantry.Items {
		meta := w.MetaForPantryItem(it)
		if meta.BoxID != "" {
			if _, exists := w.FindPantryBox(meta.BoxID); exists {
				continue
			}
		}
		zk, _, _ := pantryZoneCategory(it.Location)
		if zk == box.ZoneKey {
			matchingZone = append(matchingZone, it)
		} else {
			other = append(other, it)
		}
	}
	return append(matchingZone, other...)
}

func (w *Workspace) renderPantryBoxDialog(c *ui.Context) {
	if !w.PantryBoxDialogOpen {
		ui.Box(c.Key("pantry-box-dlg")).Children(func() {
			components.Dialog(c, &w.PantryBoxDialogOpen, components.DialogOptions{
				Label: "新建储藏室收纳盒 / 离心管架",
			}, nil)
		})
		return
	}
	t := c.Theme()
	tok := theme.GetTokens(t)
	title := "新建储藏室收纳盒 / 离心管架"
	if w.boxEditingID != "" {
		title = "编辑收纳盒 · " + firstNonEmptyStr(w.boxDraft.Name, "配置")
	}

	ui.Box(c.Key("pantry-box-dlg")).Children(func() {
		components.Dialog(c, &w.PantryBoxDialogOpen, components.DialogOptions{
			Label: title,
		}, func(panel ui.Element) {
		panel.Width(float32(tok.Spacing48)*14).Padding(t.Space(4)).Radius(float32(tok.DialogRadius)).
			Background(tok.Surface).Border(1, tok.BorderStrong)

		ui.Column(c).FillWidth().Gap(t.Space(2)).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
				ui.Icon(c, lucide.Grid3x3()).Size(t.Space(4), t.Space(4)).TextColor(tok.Accent)
				ui.Text(c, title).FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text).Grow(1).MinWidth(0)
				if r, er := strconv.Atoi(strings.TrimSpace(w.boxDraftRowsStr)); er == nil {
					if col, ec := strconv.Atoi(strings.TrimSpace(w.boxDraftColsStr)); ec == nil && r > 0 && col > 0 {
						components.Badge(c, fmt.Sprintf("共 %d 孔位", r*col), components.BadgeOptions{Tone: components.BadgeAccent})
					}
				}
			})

			ui.Text(c, "选择容器规格（贴纸与默认管型自动匹配）").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
			specs := AllPantryBoxSpecs()
			const cols = 3
			for i := 0; i < len(specs); i += cols {
				end := min(i+cols, len(specs))
				ui.Row(c).FillWidth().Gap(t.Space(2)).Children(func() {
					for _, sp := range specs[i:end] {
						sp := sp
						chosen := w.boxDraft.Kind == sp.Kind
						bg := tok.Elevated
						bd := tok.Border
						if chosen {
							bg = tok.AccentSoftBg
							bd = tok.Accent
						}
						tile := ui.ButtonBase(c.Key("box-spec-"+string(sp.Kind))).Label("选用 "+sp.Title).
							Grow(1).Basis(0).MinWidth(0).Padding(t.Space(2)).
							Radius(float32(tok.ControlRadius)).Background(bg).Border(1, bd).
							OnClick(func() {
								w.selectBoxDraftPreset(sp.Kind)
							})
						tile.Children(func() {
							ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
								labsticker.RenderSticker(c, sp.StickerID, nil, labsticker.StickerOptions{
									TargetHeightDIP: 42,
									BorderDIP:       3.0,
								})
								ui.Column(c).Grow(1).MinWidth(0).Gap(t.Space(0)).Children(func() {
									ui.Text(c, sp.ShortTitle).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Text).
										SingleLine().Ellipsis("…")
									ui.Text(c, sp.TubeHint).
										FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine().Ellipsis("…")
								})
							})
						})
					}
				})
			}

			ui.Row(c).FillWidth().Gap(t.Space(2)).Children(func() {
				ui.Column(c).Grow(2).MinWidth(0).Gap(t.Space(1)).Children(func() {
					ui.Text(c, "盒子名称 *").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
					components.TextBox(c, &w.boxDraft.Name, components.TextBoxOptions{}).Placeholder("如 抗体冻存盒 #1 / qPCR 引物 96 孔盒")
				})
				// UI-10: PCR 96-well box has fixed standard 8×12 dimensions; other boxes let user edit rows/cols freely.
				if w.boxDraft.Kind != BoxPCR96 {
					ui.Column(c).Width(float32(tok.Spacing48)*2.0).Gap(t.Space(1)).Children(func() {
						ui.Text(c, "行数 (2–12)").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
						components.TextBox(c, &w.boxDraftRowsStr, components.TextBoxOptions{})
					})
					ui.Column(c).Width(float32(tok.Spacing48)*2.0).Gap(t.Space(1)).Children(func() {
						ui.Text(c, "列数 (2–12)").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
						components.TextBox(c, &w.boxDraftColsStr, components.TextBoxOptions{})
					})
				}
			})

			ui.Row(c).FillWidth().AlignItems(ui.End).Gap(t.Space(2)).Children(func() {
				ui.Column(c).Gap(t.Space(1)).Children(func() {
					ui.Text(c, "所属温区货架").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
					zones := []struct{ k, lbl string }{
						{"-80", "-80°C"},
						{"-20", "-20°C"},
						{"4C", "4°C"},
						{"ambient", "常温避光"},
					}
					ui.Row(c).Gap(t.Space(1)).Children(func() {
						for _, z := range zones {
							z := z
							variant := components.ButtonGhost
							if w.boxDraft.ZoneKey == z.k {
								variant = components.ButtonSecondary
							}
							if components.Button(c, z.lbl, variant).Clicked() {
								w.boxDraft.ZoneKey = z.k
							}
						}
					})
				})
				ui.Column(c).Grow(1).MinWidth(0).Gap(t.Space(1)).Children(func() {
					ui.Text(c, "架层补充位置").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
					components.TextBox(c, &w.boxDraft.ShelfPos, components.TextBoxOptions{}).Placeholder("如 二层左 / 门架顶格")
				})
			})

			if w.boxDialogErr != "" {
				ui.Row(c).FillWidth().Padding(t.Space(2), t.Space(3)).
					Radius(float32(tok.ControlRadius)).Background(tok.DangerBg).Border(1, tok.Danger).Children(func() {
					ui.Text(c, w.boxDialogErr).FontSize(float32(tok.TextMeta)).TextColor(tok.DangerText)
				})
			}

			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
				if w.boxEditingID != "" {
					if !w.boxConfirmDel {
						if components.Button(c, "删除收纳盒", components.ButtonGhost).Clicked() {
							w.boxConfirmDel = true
						}
					} else {
						ui.Text(c, "盒内试剂将退回货架，不会被删除：").FontSize(float32(tok.TextMeta)).TextColor(tok.DangerText)
						if components.Button(c, "确认移除盒子", components.ButtonSecondary).Clicked() {
							_ = w.DeletePantryBox(w.boxEditingID)
							w.PantryBoxDialogOpen = false
							w.boxConfirmDel = false
						}
						if components.Button(c.Key("box-del-cancel"), "取消", components.ButtonGhost).Clicked() {
							w.boxConfirmDel = false
						}
					}
				}
				ui.Spacer(c)
				if components.Button(c.Key("box-dlg-cancel"), "取消", components.ButtonSecondary).Clicked() {
					w.PantryBoxDialogOpen = false
					w.boxConfirmDel = false
					w.boxDialogErr = ""
				}
				if components.Button(c.Key("box-dlg-save"), "保存收纳盒", components.ButtonPrimary).Clicked() {
					_ = w.SavePantryBoxDialog()
				}
			})
		})
	})
	})
}

func detailStatusTone(status string) components.BadgeTone {
	switch status {
	case "near_expiry":
		return components.BadgeWarning
	case "expired", "exhausted":
		return components.BadgeDanger
	case "opened":
		return components.BadgeAccent
	default:
		return components.BadgeSuccess
	}
}

func boxStatusTextColor(tok theme.Tokens, status string) ui.Color {
	switch status {
	case "near_expiry":
		return tok.WarningText
	case "expired", "exhausted":
		return tok.DangerText
	default:
		return tok.TextMuted
	}
}

func truncateRunes(s string, maxLen int) string {
	r := []rune(strings.TrimSpace(s))
	if maxLen <= 0 || len(r) <= maxLen {
		return string(r)
	}
	return string(r[:maxLen]) + "…"
}
