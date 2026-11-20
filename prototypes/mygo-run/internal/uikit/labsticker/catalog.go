package labsticker

import (
	"embed"
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"
)

//go:embed assets/*.svg
var assetFS embed.FS

// VesselID identifies one of the 12 curated BioRender-style laboratory vessel stickers.
type VesselID string

const (
	VesselPCRStrip         VesselID = "pcr_strip"
	VesselEP15             VesselID = "ep_1_5ml"
	VesselCryovial2        VesselID = "cryovial_2ml"
	VesselScrewTube2       VesselID = "screw_tube_2ml"
	VesselAmberMicrovial   VesselID = "amber_microvial"
	VesselSerumVial        VesselID = "serum_vial"
	VesselFalcon15         VesselID = "falcon_15ml"
	VesselFalcon50         VesselID = "falcon_50ml"
	VesselMediaBottle500   VesselID = "media_bottle_500ml"
	VesselReagentGL45      VesselID = "reagent_bottle_gl45"
	VesselAmberBottle500   VesselID = "amber_bottle_500ml"
	VesselErlenmeyer250    VesselID = "erlenmeyer_250ml"

	// Curated BioRender-style storage boxes & tube racks (nested containers on pantry shelves).
	VesselBoxCryo9x9  VesselID = "box_cryo_9x9"
	VesselBoxEP50     VesselID = "box_ep_50"
	VesselBoxFalcon15 VesselID = "box_falcon_15ml"
	VesselBoxFalcon50 VesselID = "box_falcon_50ml"
	VesselBoxPCR96    VesselID = "box_pcr_96"
)

// VesselSpec describes one laboratory vessel or storage box silhouette, its proportional
// shelf size, and sample domain metadata.
type VesselSpec struct {
	ID            VesselID
	Title         string
	VolumeLabel   string
	Category      string
	ExampleUsage  string
	DefaultHeight float32
	AssetPath     string
	IsBox         bool
}

var vesselSpecs = []VesselSpec{
	{
		ID:            VesselPCRStrip,
		Title:         "0.2 mL PCR 八联管",
		VolumeLabel:   "0.2 mL × 8",
		Category:      "微量扩增管",
		ExampleUsage:  "qPCR 引物预混 / RT-PCR 反应液",
		DefaultHeight: 56,
		AssetPath:     "assets/01_pcr_strip.svg",
	},
	{
		ID:            VesselEP15,
		Title:         "1.5 mL EP 离心管",
		VolumeLabel:   "1.5 mL",
		Category:      "微量离心管",
		ExampleUsage:  "通用缓冲液分装 / 质粒 DNA / dNTP",
		DefaultHeight: 68,
		AssetPath:     "assets/02_ep_1_5ml.svg",
	},
	{
		ID:            VesselCryovial2,
		Title:         "2.0 mL 冻存管",
		VolumeLabel:   "2.0 mL",
		Category:      "超低温冻存",
		ExampleUsage:  "细胞冻存液 / 慢病毒液氮冻存",
		DefaultHeight: 72,
		AssetPath:     "assets/03_cryovial_2ml.svg",
	},
	{
		ID:            VesselScrewTube2,
		Title:         "2.0 mL 螺口酶/引物管",
		VolumeLabel:   "2.0 mL",
		Category:      "微量管",
		ExampleUsage:  "T4 连接酶 / Taq 聚合酶 / SYBR Green",
		DefaultHeight: 72,
		AssetPath:     "assets/04_screw_tube_2ml.svg",
	},
	{
		ID:            VesselAmberMicrovial,
		Title:         "避光抗体/染料小管",
		VolumeLabel:   "5 mL",
		Category:      "避光微量瓶",
		ExampleUsage:  "一抗 / 荧光二抗 / DAPI / EdU 染料",
		DefaultHeight: 70,
		AssetPath:     "assets/05_amber_microvial.svg",
	},
	{
		ID:            VesselSerumVial,
		Title:         "冻干粉/西林小瓶",
		VolumeLabel:   "10 mL",
		Category:      "西林药瓶",
		ExampleUsage:  "抗生素标准品 / 冻干生长因子 (VEGF)",
		DefaultHeight: 74,
		AssetPath:     "assets/06_serum_vial.svg",
	},
	{
		ID:            VesselFalcon15,
		Title:         "15 mL 锥底离心管",
		VolumeLabel:   "15 mL",
		Category:      "锥底离心管",
		ExampleUsage:  "胰酶消化液 / 10× PBS / 细胞悬液分装",
		DefaultHeight: 88,
		AssetPath:     "assets/07_falcon_15ml.svg",
	},
	{
		ID:            VesselFalcon50,
		Title:         "50 mL 锥底离心管",
		VolumeLabel:   "50 mL",
		Category:      "锥底离心管",
		ExampleUsage:  "胎牛血清 (FBS) / 青链霉素双抗 / 胰酶母液",
		DefaultHeight: 92,
		AssetPath:     "assets/08_falcon_50ml.svg",
	},
	{
		ID:            VesselMediaBottle500,
		Title:         "500 mL 培养基瓶",
		VolumeLabel:   "500 mL",
		Category:      "培养基瓶",
		ExampleUsage:  "DMEM 高糖培养基 / RPMI-1640 培养基",
		DefaultHeight: 96,
		AssetPath:     "assets/09_media_bottle_500ml.svg",
	},
	{
		ID:            VesselReagentGL45,
		Title:         "GL45 蓝盖玻璃试剂瓶",
		VolumeLabel:   "500 mL",
		Category:      "标准试剂瓶",
		ExampleUsage:  "1× PBS 缓冲液 / TAE 电泳缓冲液 / ddH₂O",
		DefaultHeight: 96,
		AssetPath:     "assets/10_reagent_bottle_gl45.svg",
	},
	{
		ID:            VesselAmberBottle500,
		Title:         "500 mL 棕色避光试剂瓶",
		VolumeLabel:   "500 mL",
		Category:      "避光试剂瓶",
		ExampleUsage:  "4% 多聚甲醛 (PFA) / TRIzol / ECL 发光液",
		DefaultHeight: 96,
		AssetPath:     "assets/11_amber_bottle_500ml.svg",
	},
	{
		ID:            VesselErlenmeyer250,
		Title:         "250 mL 三角摇瓶",
		VolumeLabel:   "250 mL",
		Category:      "摇瓶/配液瓶",
		ExampleUsage:  "LB 无菌培养基 / 无菌菌液摇床扩大培养",
		DefaultHeight: 92,
		AssetPath:     "assets/12_erlenmeyer_250ml.svg",
	},
}

var boxSpecs = []VesselSpec{
	{
		ID:            VesselBoxCryo9x9,
		Title:         "9×9 / 10×10 冻存盒",
		VolumeLabel:   "81 / 100 孔",
		Category:      "超低温冻存盒",
		ExampleUsage:  "-80°C 一抗原液 / 慢病毒 / 冻存细胞株",
		DefaultHeight: 84,
		AssetPath:     "assets/13_cryobox_9x9.svg",
		IsBox:         true,
	},
	{
		ID:            VesselBoxEP50,
		Title:         "1.5 / 2.0 mL EP 离心管盒",
		VolumeLabel:   "5×10 孔",
		Category:      "微量离心管盒",
		ExampleUsage:  "-20°C 分子酶 / 引物分装 / 质粒 DNA",
		DefaultHeight: 82,
		AssetPath:     "assets/14_ep_tube_box.svg",
		IsBox:         true,
	},
	{
		ID:            VesselBoxFalcon15,
		Title:         "15 mL 离心管架",
		VolumeLabel:   "5×5 孔",
		Category:      "锥底离心管架",
		ExampleUsage:  "4°C 胰酶 EDTA / 双抗 / 分装悬液",
		DefaultHeight: 88,
		AssetPath:     "assets/15_falcon_rack_15ml.svg",
		IsBox:         true,
	},
	{
		ID:            VesselBoxFalcon50,
		Title:         "50 mL 离心管架",
		VolumeLabel:   "4×4 孔",
		Category:      "大管离心管架",
		ExampleUsage:  "4°C 胎牛血清分装 / 10× PBS 母液",
		DefaultHeight: 90,
		AssetPath:     "assets/16_falcon_rack_50ml.svg",
		IsBox:         true,
	},
	{
		ID:            VesselBoxPCR96,
		Title:         "96 孔 0.2 mL PCR 盒",
		VolumeLabel:   "8×12 孔",
		Category:      "PCR 扩增管盒",
		ExampleUsage:  "-20°C qPCR 八联排引物 / cDNA 文库",
		DefaultHeight: 82,
		AssetPath:     "assets/17_pcr_box_96.svg",
		IsBox:         true,
	},
}

var (
	parsedSVGsMu sync.Mutex
	parsedSVGs   = map[VesselID]*ui.SVG{}
	vesselByID   = map[VesselID]VesselSpec{}
)

func init() {
	for _, spec := range vesselSpecs {
		vesselByID[spec.ID] = spec
	}
	for _, spec := range boxSpecs {
		vesselByID[spec.ID] = spec
	}
}

// AllVessels returns the 12 curated BioRender-style reagent vessel presets in display order.
func AllVessels() []VesselSpec {
	out := make([]VesselSpec, len(vesselSpecs))
	copy(out, vesselSpecs)
	return out
}

// AllBoxes returns the 5 curated BioRender-style storage box & rack presets in display order.
func AllBoxes() []VesselSpec {
	out := make([]VesselSpec, len(boxSpecs))
	copy(out, boxSpecs)
	return out
}

// Lookup returns the VesselSpec for id if present.
func Lookup(id VesselID) (VesselSpec, bool) {
	spec, ok := vesselByID[id]
	return spec, ok
}

// ParseVesselSVG returns the cached parsed *ui.SVG for id.
func ParseVesselSVG(id VesselID) (*ui.SVG, error) {
	spec, ok := Lookup(id)
	if !ok {
		return nil, fmt.Errorf("labsticker: unknown vessel id %q", id)
	}
	parsedSVGsMu.Lock()
	if s, ok := parsedSVGs[id]; ok {
		parsedSVGsMu.Unlock()
		return s, nil
	}
	parsedSVGsMu.Unlock()

	raw, err := assetFS.ReadFile(spec.AssetPath)
	if err != nil {
		return nil, fmt.Errorf("labsticker: read %s: %w", spec.AssetPath, err)
	}
	parsed, err := ui.ParseSVG(raw)
	if err != nil {
		return nil, fmt.Errorf("labsticker: parse %s: %w", spec.AssetPath, err)
	}
	parsedSVGsMu.Lock()
	if existing, ok := parsedSVGs[id]; ok {
		parsedSVGsMu.Unlock()
		return existing, nil
	}
	parsedSVGs[id] = parsed
	parsedSVGsMu.Unlock()
	return parsed, nil
}

// Sticker returns a cached die-cut white-bordered *ui.Bitmap and layout metrics
// for the given vessel ID.
func Sticker(id VesselID, opts StickerOptions) (*ui.Bitmap, StickerMetrics, error) {
	spec, ok := Lookup(id)
	if !ok {
		return nil, StickerMetrics{}, fmt.Errorf("labsticker: unknown vessel id %q", id)
	}
	hDIP := opts.TargetHeightDIP
	if hDIP <= 0 {
		hDIP = spec.DefaultHeight
	}
	bDIP := opts.BorderDIP
	if bDIP <= 0 {
		bDIP = defaultBorder
	}
	hPx := max(16, int(math.Round(float64(hDIP))))
	bPx := max(1, int(math.Round(float64(bDIP)*10)))

	stickerCacheMu.Lock()
	key := stickerCacheKey{id: id, heightPx: hPx, borderPx: bPx, exhausted: opts.Exhausted}
	if hit, ok := stickerCache[key]; ok {
		stickerCacheMu.Unlock()
		return hit.bmp, hit.metrics, nil
	}
	stickerCacheMu.Unlock()

	s, err := ParseVesselSVG(id)
	if err != nil {
		return nil, StickerMetrics{}, err
	}
	bmp, metrics := composeSVGSticker(s, float32(hPx), float32(bDIP), opts.Exhausted)
	if bmp == nil {
		return nil, StickerMetrics{}, fmt.Errorf("labsticker: empty rasterization for %s", id)
	}
	stickerCacheMu.Lock()
	if hit, ok := stickerCache[key]; ok {
		stickerCacheMu.Unlock()
		return hit.bmp, hit.metrics, nil
	}
	stickerCache[key] = cachedSticker{bmp: bmp, metrics: metrics}
	stickerCacheMu.Unlock()
	return bmp, metrics, nil
}

// SuggestVesselID infers the best-matching container sticker from reagent names
// and storage conditions.
func SuggestVesselID(name, location string) VesselID {
	s := strings.ToLower(name + " " + location)
	switch {
	case strings.Contains(s, "pcr") && (strings.Contains(s, "条") || strings.Contains(s, "联") || strings.Contains(s, "strip") || strings.Contains(s, "0.2")):
		return VesselPCRStrip
	case strings.Contains(s, "dmem") || strings.Contains(s, "rpmi") || strings.Contains(s, "培养基"):
		if strings.Contains(s, "lb") || strings.Contains(s, "摇") {
			return VesselErlenmeyer250
		}
		return VesselMediaBottle500
	case strings.Contains(s, "pfa") || strings.Contains(s, "多聚甲醛") || strings.Contains(s, "trizol") || strings.Contains(s, "ecl") || strings.Contains(s, "避光") && strings.Contains(s, "瓶"):
		return VesselAmberBottle500
	case strings.Contains(s, "抗体") || strings.Contains(s, "antibody") || strings.Contains(s, "dapi") || strings.Contains(s, "edu") || strings.Contains(s, "fitc") || strings.Contains(s, "染料") || strings.Contains(s, "避光"):
		return VesselAmberMicrovial
	case strings.Contains(s, "冻干") || strings.Contains(s, "标准品") || strings.Contains(s, "vegf") || strings.Contains(s, "万古霉素") || strings.Contains(s, "抗生素"):
		return VesselSerumVial
	case strings.Contains(s, "fbs") || strings.Contains(s, "血清") || strings.Contains(s, "50ml") || strings.Contains(s, "50 ml"):
		return VesselFalcon50
	case strings.Contains(s, "15ml") || strings.Contains(s, "15 ml") || strings.Contains(s, "胰酶") || strings.Contains(s, "trypsin"):
		return VesselFalcon15
	case strings.Contains(s, "pbs") || strings.Contains(s, "tae") || strings.Contains(s, "buffer") || strings.Contains(s, "缓冲"):
		return VesselReagentGL45
	case strings.Contains(s, "冻存") || strings.Contains(s, "-80"):
		return VesselCryovial2
	case strings.Contains(s, "酶") || strings.Contains(s, "sybr") || strings.Contains(s, "taq") || strings.Contains(s, "引物") || strings.Contains(s, "primer"):
		return VesselScrewTube2
	case strings.Contains(s, "摇瓶") || strings.Contains(s, "lb"):
		return VesselErlenmeyer250
	default:
		return VesselEP15
	}
}
