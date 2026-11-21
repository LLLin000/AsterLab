package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"asterlab-mygo-prototype/modules/runs"
)

const pantryNearExpiryWindowDays uint32 = 30

type localPantryDisk struct {
	NextSeq int               `json:"nextSeq"`
	Items   []runs.PantryItem `json:"items"`
}

func (a *app) pantryLedgerPath() string {
	dir := workspaceDataDir()
	if dir != "" {
		return filepath.Join(dir, "pantry-ledger.json")
	}
	if a.vault != nil && a.vault.Root != "" {
		return filepath.Join(a.vault.Root, ".asterlab", "pantry-ledger.json")
	}
	return ""
}

func loadLocalPantryDisk(path string) (localPantryDisk, error) {
	if path == "" {
		return localPantryDisk{}, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return localPantryDisk{}, nil
		}
		return localPantryDisk{}, err
	}
	var disk localPantryDisk
	if err := json.Unmarshal(raw, &disk); err != nil {
		return localPantryDisk{}, err
	}
	if disk.Items == nil {
		disk.Items = []runs.PantryItem{}
	}
	return disk, nil
}

func saveLocalPantryDisk(path string, disk localPantryDisk) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(disk, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0644)
}

func computePantryDerived(it runs.PantryItem, today time.Time) runs.PantryItem {
	it.ExpiresOn = ""
	it.DaysRemaining = nil
	it.ExpiryBasis = ""

	var anchor string
	var shelf uint32
	if strings.TrimSpace(it.OpenedOn) != "" && it.OpenedShelfLifeDays != nil && *it.OpenedShelfLifeDays > 0 {
		anchor = strings.TrimSpace(it.OpenedOn)
		shelf = *it.OpenedShelfLifeDays
		it.ExpiryBasis = "opened_shelf_life"
	} else if it.ShelfLifeDays != nil && *it.ShelfLifeDays > 0 {
		anchor = firstNonEmpty(strings.TrimSpace(it.PurchasedOn), strings.TrimSpace(it.OpenedOn))
		shelf = *it.ShelfLifeDays
		if anchor != "" {
			it.ExpiryBasis = "shelf_life"
		}
	}

	if anchor != "" && shelf > 0 {
		if baseDate, err := time.Parse("2006-01-02", anchor); err == nil {
			expDate := baseDate.AddDate(0, 0, int(shelf))
			it.ExpiresOn = expDate.Format("2006-01-02")
			rem := int64(expDate.Sub(today).Hours() / 24)
			it.DaysRemaining = &rem
		}
	}

	switch {
	case it.Exhausted:
		it.Status = "exhausted"
	case it.DaysRemaining != nil && *it.DaysRemaining < 0:
		it.Status = "expired"
	case it.DaysRemaining != nil && *it.DaysRemaining <= int64(pantryNearExpiryWindowDays):
		it.Status = "near_expiry"
	case strings.TrimSpace(it.OpenedOn) != "":
		it.Status = "opened"
	default:
		it.Status = "unopened"
	}
	return it
}

func (a *app) localListPantry() (runs.PantryLedger, error) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	disk, err := loadLocalPantryDisk(a.pantryLedgerPath())
	if err != nil {
		return runs.PantryLedger{}, err
	}
	items := make([]runs.PantryItem, 0, len(disk.Items))
	for _, it := range disk.Items {
		items = append(items, computePantryDerived(it, today))
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].UpdatedAt != items[j].UpdatedAt {
			return items[i].UpdatedAt > items[j].UpdatedAt
		}
		return items[i].Name < items[j].Name
	})
	return runs.PantryLedger{
		AsOf:           today.Format("2006-01-02"),
		NearExpiryDays: pantryNearExpiryWindowDays,
		Items:          items,
	}, nil
}

func (a *app) localUpsertPantry(input runs.PantryUpsert) (runs.PantryItem, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return runs.PantryItem{}, fmt.Errorf("请输入试剂或耗材名称")
	}
	path := a.pantryLedgerPath()
	disk, err := loadLocalPantryDisk(path)
	if err != nil {
		return runs.PantryItem{}, err
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	nowRFC := now.UTC().Format(time.RFC3339)

	idx := -1
	if strings.TrimSpace(input.ID) != "" {
		for i := range disk.Items {
			if disk.Items[i].ID == input.ID {
				idx = i
				break
			}
		}
		if idx < 0 {
			return runs.PantryItem{}, fmt.Errorf("未找到试剂条目：%s", input.ID)
		}
		if input.ExpectedRevision != nil && disk.Items[idx].Revision != *input.ExpectedRevision {
			return runs.PantryItem{}, fmt.Errorf("CONFLICT: 试剂条目已被其他操作更新，请刷新后重试")
		}
	}

	var saved runs.PantryItem
	if idx >= 0 {
		prev := disk.Items[idx]
		saved = runs.PantryItem{
			ID:                  prev.ID,
			LabelCode:           prev.LabelCode,
			ProjectID:           firstNonEmpty(input.ProjectID, prev.ProjectID),
			Name:                name,
			Location:            strings.TrimSpace(input.Location),
			PurchasedOn:         strings.TrimSpace(input.PurchasedOn),
			OpenedOn:            strings.TrimSpace(input.OpenedOn),
			ShelfLifeDays:       input.ShelfLifeDays,
			OpenedShelfLifeDays: input.OpenedShelfLifeDays,
			Exhausted:           input.Exhausted,
			Revision:            prev.Revision + 1,
			CreatedAt:           prev.CreatedAt,
			UpdatedAt:           nowRFC,
		}
		saved = computePantryDerived(saved, today)
		disk.Items[idx] = saved
	} else {
		disk.NextSeq++
		seq := disk.NextSeq
		saved = runs.PantryItem{
			ID:                  fmt.Sprintf("pantry-%d-%d", now.UnixMilli(), seq),
			LabelCode:           fmt.Sprintf("PN-%03d", seq),
			ProjectID:           input.ProjectID,
			Name:                name,
			Location:            strings.TrimSpace(input.Location),
			PurchasedOn:         strings.TrimSpace(input.PurchasedOn),
			OpenedOn:            strings.TrimSpace(input.OpenedOn),
			ShelfLifeDays:       input.ShelfLifeDays,
			OpenedShelfLifeDays: input.OpenedShelfLifeDays,
			Exhausted:           input.Exhausted,
			Revision:            1,
			CreatedAt:           nowRFC,
			UpdatedAt:           nowRFC,
		}
		saved = computePantryDerived(saved, today)
		disk.Items = append(disk.Items, saved)
	}
	if err := saveLocalPantryDisk(path, disk); err != nil {
		return runs.PantryItem{}, err
	}
	return saved, nil
}

func (a *app) localDeletePantry(input runs.PantryDelete) error {
	if strings.TrimSpace(input.ID) == "" {
		return fmt.Errorf("缺少待删除试剂 ID")
	}
	path := a.pantryLedgerPath()
	disk, err := loadLocalPantryDisk(path)
	if err != nil {
		return err
	}
	idx := -1
	for i := range disk.Items {
		if disk.Items[i].ID == input.ID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("未找到试剂条目：%s", input.ID)
	}
	if input.ExpectedRevision != nil && disk.Items[idx].Revision != *input.ExpectedRevision {
		return fmt.Errorf("CONFLICT: 试剂条目已被修改，请刷新后重试")
	}
	disk.Items = append(disk.Items[:idx], disk.Items[idx+1:]...)
	return saveLocalPantryDisk(path, disk)
}

func (a *app) localExportPantryLabels() (runs.PantryLabels, error) {
	ledger, err := a.localListPantry()
	if err != nil {
		return runs.PantryLabels{}, err
	}
	labels := make([]runs.PantryLabel, 0, len(ledger.Items))
	for _, it := range ledger.Items {
		labels = append(labels, runs.PantryLabel{
			LabelCode:     it.LabelCode,
			PantryItemID:  it.ID,
			Name:          it.Name,
			Location:      it.Location,
			Status:        it.Status,
			ExpiresOn:     it.ExpiresOn,
			DaysRemaining: it.DaysRemaining,
			Revision:      it.Revision,
		})
	}
	return runs.PantryLabels{
		SchemaVersion:  1,
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		AsOf:           ledger.AsOf,
		NearExpiryDays: ledger.NearExpiryDays,
		Labels:         labels,
	}, nil
}
