package notebook

import (
	"reflect"
	"strings"
	"testing"

	"asterlab-mygo-prototype/internal/resultgrid"
	"asterlab-mygo-prototype/modules/runs"
	"github.com/egoist/mygo/ui/richtext"
)

func TestCitationInsertionAndSnapshotResolution(t *testing.T) {
	snap := fixture()
	snap.Measurements = []runs.Measurement{{SampleID: "sample-1", Endpoint: "OD450", Value: .82, Unit: "AU"}}
	asset := runs.Asset{ID: "asset-1", RelativePath: "data/run.csv", SHA256: "abc", Availability: "available", Link: runs.AssetLink{RunID: snap.RunID}}
	items := []Citation{NewEventCitation(snap.Events[2]), NewAssetCitation(asset), NewReadoutCitation(snap.Measurements[0])}
	var markdown strings.Builder
	for _, item := range items { markdown.WriteString(SerializeCitation(item)) }
	resolved := ResolveCitations(markdown.String(), snap, []runs.Asset{asset})
	if len(resolved) != 3 { t.Fatalf("resolved %d citations", len(resolved)) }
	for i, item := range resolved {
		if item.Status != "current" || item.Citation.Kind != items[i].Kind || item.Citation.ID != items[i].ID {
			t.Fatalf("citation %d resolved as %+v", i, item)
		}
	}
	if got := ParseCitations("[event evt-legacy] [batch batch-1]"); len(got) != 2 || got[0].Kind != "event" || got[1].Kind != "batch" {
		t.Fatalf("legacy tokens = %+v", got)
	}
}

func TestCitationStalenessAndMissingAssets(t *testing.T) {
	snap := fixture()
	snap.Measurements = []runs.Measurement{{SampleID: "sample-1", Endpoint: "OD450", Value: .82, Unit: "AU"}}
	event := snap.Events[2]
	asset := runs.Asset{ID: "asset-1", RelativePath: "data/run.csv", SHA256: "abc", Availability: "available", Link: runs.AssetLink{RunID: snap.RunID}}
	markdown := SerializeCitation(NewEventCitation(event)) + SerializeCitation(NewReadoutCitation(snap.Measurements[0])) + SerializeCitation(NewAssetCitation(asset))
	changed := snap
	changed.Events = append([]runs.RunEvent(nil), snap.Events...)
	changed.Events[2].Content = "edited"
	changed.Measurements = append([]runs.Measurement(nil), snap.Measurements...)
	changed.Measurements[0].Value = .9
	changedAsset := asset
	changedAsset.SHA256 = "def"
	resolved := ResolveCitations(markdown, changed, []runs.Asset{changedAsset})
	for _, item := range resolved { if item.Status != "stale" { t.Errorf("%s status = %q, want stale", item.Citation.Kind, item.Status) } }
	resolved = ResolveCitations(markdown, snap, nil)
	if resolved[2].Status != "missing" { t.Fatalf("absent asset status = %q", resolved[2].Status) }
	asset.Availability = "unavailable"
	if got := ResolveCitations(markdown, snap, []runs.Asset{asset})[2].Status; got != "missing" { t.Fatalf("unavailable asset status = %q", got) }
	asset.Availability = "available"
	asset.Link.RunID = "another-run"
	if got := ResolveCitations(markdown, snap, []runs.Asset{asset})[2].Status; got != "stale" { t.Fatalf("unlinked asset status = %q", got) }
}

func TestRemoveCitationOnlyChangesMarkdown(t *testing.T) {
	snap := fixture()
	measurement := runs.Measurement{SampleID: "sample-1", Endpoint: "OD450", Value: .82, Unit: "AU"}
	snap.Measurements = []runs.Measurement{measurement}
	asset := runs.Asset{ID: "asset-1", RelativePath: "data/run.csv", SHA256: "abc", Availability: "available", Link: runs.AssetLink{RunID: snap.RunID}}
	beforeSnap := snap
	beforeAsset := asset
	markdown := "keep " + SerializeCitation(NewEventCitation(snap.Events[2])) + " remove " + SerializeCitation(NewAssetCitation(asset))
	updated := RemoveCitation(markdown, 1)
	if !strings.Contains(updated, SerializeCitation(NewEventCitation(snap.Events[2]))) || strings.Contains(updated, SerializeCitation(NewAssetCitation(asset))) {
		t.Fatalf("removed wrong token: %s", updated)
	}
	if !reflect.DeepEqual(snap, beforeSnap) || !reflect.DeepEqual(asset, beforeAsset) { t.Fatal("removing citation changed authoritative data") }
	if got := RemoveCitation(updated, -1); got != updated { t.Fatal("invalid index changed Markdown") }
}

func TestAtMentionTriggerCatalogAndRichtextRoundTrip(t *testing.T) {
	if q, ok := ExtractAtQuery("请看 @板图"); !ok || q != "板图" {
		t.Fatalf("ExtractAtQuery failed: q=%q ok=%v", q, ok)
	}
	if _, ok := ExtractAtQuery("mail@example.com"); ok {
		t.Fatal("email address should not trigger @mention")
	}

	snap := fixture()
	snap.Title = "CCK-8 细胞增殖实验"
	snap.Tasks = []runs.RunTask{{Key: "step-read", Title: "酶标仪读取", Path: "root/read", Timepoint: "24 h", Status: "completed"}}
	snap.Measurements = []runs.Measurement{{SampleID: "S1", Endpoint: "OD450", Value: 0.82, Unit: "AU"}}
	asset := runs.Asset{ID: "asset-1", RelativePath: "data/plate-raw.csv", SHA256: "abc", Availability: "available", Link: runs.AssetLink{RunID: snap.RunID}}
	frozen := "```asterlab\nkind: plate\nid: plate-96\nschema: 1\nformat: 96\nassay: CCK-8\ngroups:\n  ctrl: [A1, A2]\n```\n"
	grid := &resultgrid.Grid{
		Schema: resultgrid.Schema{ID: "plate-96", RowLabels: []string{"A"}, ColumnLabels: []string{"1"}},
	}

	cat := BuildCitationCatalog("肿瘤药敏项目", snap, []runs.Asset{asset}, grid, frozen, "")
	if cat.Project != "肿瘤药敏项目" {
		t.Fatalf("project = %q", cat.Project)
	}
	wantGroups := map[string]bool{"plate": false, "summary": false, "event": false, "step": false, "asset": false, "readout": false}
	for _, g := range cat.Groups {
		if len(g.Items) > 0 {
			wantGroups[g.Key] = true
		}
	}
	for k, seen := range wantGroups {
		if !seen {
			t.Fatalf("missing catalog group %q in %+v", k, cat.Groups)
		}
	}

	plateCat := BuildCitationCatalog("肿瘤药敏项目", snap, []runs.Asset{asset}, grid, frozen, "plate-96")
	if len(plateCat.Groups) != 1 || plateCat.Groups[0].Key != "plate" {
		t.Fatalf("filtered catalog = %+v", plateCat.Groups)
	}

	note := ApplyAtCitation("请核对 @plate-96", plateCat.Groups[0].Items[0].Citation)
	rt := richtext.NewDocument(richtext.ParseMarkdown(note)).Text().Markdown()
	resolved := ResolveCitationsWith(rt, snap, []runs.Asset{asset}, grid, frozen)
	if len(resolved) != 1 || resolved[0].Status != "current" || resolved[0].Citation.Kind != "plate" {
		t.Fatalf("richtext round-trip resolved = %+v (rt=%q)", resolved, rt)
	}
}
