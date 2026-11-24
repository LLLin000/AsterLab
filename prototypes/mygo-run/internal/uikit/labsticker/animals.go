package labsticker

import (
	"fmt"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"
)

// AnimalSpeciesID identifies one curated BioRender-style laboratory animal vector sticker.
type AnimalSpeciesID string

const (
	AnimalMouseWhite AnimalSpeciesID = "mouse_white"
	AnimalMouseBlack AnimalSpeciesID = "mouse_black"
	AnimalMouseNude  AnimalSpeciesID = "mouse_nude"
	AnimalRat        AnimalSpeciesID = "rat"
	AnimalRabbit     AnimalSpeciesID = "rabbit"
	AnimalBeagle     AnimalSpeciesID = "beagle"
	AnimalGuineaPig  AnimalSpeciesID = "guinea_pig"
	AnimalZebrafish  AnimalSpeciesID = "zebrafish"
)

// AnimalSpec describes one laboratory animal species model for vivarium tracking.
type AnimalSpec struct {
	ID                AnimalSpeciesID
	Title             string
	CommonStrains     string
	DefaultGrade      string
	DefaultMaxPerCage int
	DefaultUnit       string
	AssetPath         string
}

var animalSpecs = []AnimalSpec{
	{
		ID:                AnimalMouseWhite,
		Title:             "实验白小鼠",
		CommonStrains:     "BALB/c · ICR · KM",
		DefaultGrade:      "SPF",
		DefaultMaxPerCage: 5,
		DefaultUnit:       "只",
		AssetPath:         "assets/20_animal_mouse_white.svg",
	},
	{
		ID:                AnimalMouseBlack,
		Title:             "实验黑小鼠",
		CommonStrains:     "C57BL/6J · ApoE-/-",
		DefaultGrade:      "SPF",
		DefaultMaxPerCage: 5,
		DefaultUnit:       "只",
		AssetPath:         "assets/21_animal_mouse_black.svg",
	},
	{
		ID:                AnimalMouseNude,
		Title:             "免疫缺陷裸鼠",
		CommonStrains:     "BALB/c-nu · NCG · NOD-SCID",
		DefaultGrade:      "SPF",
		DefaultMaxPerCage: 4,
		DefaultUnit:       "只",
		AssetPath:         "assets/22_animal_mouse_nude.svg",
	},
	{
		ID:                AnimalRat,
		Title:             "实验大鼠",
		CommonStrains:     "SD · Wistar · SHR",
		DefaultGrade:      "SPF",
		DefaultMaxPerCage: 3,
		DefaultUnit:       "只",
		AssetPath:         "assets/23_animal_rat.svg",
	},
	{
		ID:                AnimalRabbit,
		Title:             "实验兔",
		CommonStrains:     "新西兰白兔 · 日本大耳白兔",
		DefaultGrade:      "conventional",
		DefaultMaxPerCage: 1,
		DefaultUnit:       "只",
		AssetPath:         "assets/24_animal_rabbit.svg",
	},
	{
		ID:                AnimalBeagle,
		Title:             "实验犬（比格犬）",
		CommonStrains:     "Beagle 比格犬",
		DefaultGrade:      "conventional",
		DefaultMaxPerCage: 1,
		DefaultUnit:       "只",
		AssetPath:         "assets/25_animal_beagle.svg",
	},
	{
		ID:                AnimalGuineaPig,
		Title:             "豚鼠",
		CommonStrains:     "Hartley 豚鼠",
		DefaultGrade:      "conventional",
		DefaultMaxPerCage: 2,
		DefaultUnit:       "只",
		AssetPath:         "assets/26_animal_guinea_pig.svg",
	},
	{
		ID:                AnimalZebrafish,
		Title:             "斑马鱼",
		CommonStrains:     "AB · TU · Tg 荧光系",
		DefaultGrade:      "aquatic",
		DefaultMaxPerCage: 30,
		DefaultUnit:       "尾",
		AssetPath:         "assets/27_animal_zebrafish.svg",
	},
}

var (
	parsedAnimalSVGsMu sync.Mutex
	parsedAnimalSVGs   = map[AnimalSpeciesID]*ui.SVG{}
)

// AllAnimals returns all curated laboratory animal species specs in display order.
func AllAnimals() []AnimalSpec {
	out := make([]AnimalSpec, len(animalSpecs))
	copy(out, animalSpecs)
	return out
}

// LookupAnimal returns metadata for a laboratory animal species ID.
func LookupAnimal(id AnimalSpeciesID) (AnimalSpec, bool) {
	for _, spec := range animalSpecs {
		if spec.ID == id {
			return spec, true
		}
	}
	return AnimalSpec{}, false
}

// ParseAnimalSVG loads and caches the parsed vector graphic for species id.
func ParseAnimalSVG(id AnimalSpeciesID) (*ui.SVG, error) {
	spec, ok := LookupAnimal(id)
	if !ok {
		return nil, fmt.Errorf("labsticker: unknown animal species %q", id)
	}
	parsedAnimalSVGsMu.Lock()
	if s, ok := parsedAnimalSVGs[id]; ok {
		parsedAnimalSVGsMu.Unlock()
		return s, nil
	}
	parsedAnimalSVGsMu.Unlock()

	raw, err := assetFS.ReadFile(spec.AssetPath)
	if err != nil {
		return nil, fmt.Errorf("labsticker: read %s: %w", spec.AssetPath, err)
	}
	parsed, err := ui.ParseSVG(raw)
	if err != nil {
		return nil, fmt.Errorf("labsticker: parse %s: %w", spec.AssetPath, err)
	}
	parsedAnimalSVGsMu.Lock()
	if existing, ok := parsedAnimalSVGs[id]; ok {
		parsedAnimalSVGsMu.Unlock()
		return existing, nil
	}
	parsedAnimalSVGs[id] = parsed
	parsedAnimalSVGsMu.Unlock()
	return parsed, nil
}

// RenderAnimalSticker renders the crisp vector animal illustration at heightDIP (96:72 aspect).
func RenderAnimalSticker(c *ui.Context, id AnimalSpeciesID, heightDIP float32) {
	if heightDIP <= 0 {
		heightDIP = 44
	}
	svg, err := ParseAnimalSVG(id)
	if err != nil || svg == nil {
		if id != AnimalMouseBlack {
			if fallback, fErr := ParseAnimalSVG(AnimalMouseBlack); fErr == nil && fallback != nil {
				ui.Image(c, fallback).Size(heightDIP*(96.0/72.0), heightDIP)
			}
		}
		return
	}
	ui.Image(c, svg).Size(heightDIP*(96.0/72.0), heightDIP)
}

// SuggestAnimalID infers the best animal species icon from free-text strain/species names.
func SuggestAnimalID(strain string) AnimalSpeciesID {
	s := strings.ToLower(strings.TrimSpace(strain))
	switch {
	case strings.Contains(s, "裸") || strings.Contains(s, "nude") || strings.Contains(s, "ncg") || strings.Contains(s, "scid"):
		return AnimalMouseNude
	case strings.Contains(s, "c57") || strings.Contains(s, "apoe") || strings.Contains(s, "黑鼠") || strings.Contains(s, "b6"):
		return AnimalMouseBlack
	case strings.Contains(s, "大鼠") || strings.Contains(s, "rat") || strings.Contains(s, "wistar") || strings.Contains(s, "sd"):
		return AnimalRat
	case strings.Contains(s, "兔") || strings.Contains(s, "rabbit") || strings.Contains(s, "nzw"):
		return AnimalRabbit
	case strings.Contains(s, "犬") || strings.Contains(s, "狗") || strings.Contains(s, "beagle") || strings.Contains(s, "dog"):
		return AnimalBeagle
	case strings.Contains(s, "豚鼠") || strings.Contains(s, "guinea") || strings.Contains(s, "hartley"):
		return AnimalGuineaPig
	case strings.Contains(s, "斑马鱼") || strings.Contains(s, "zebrafish") || strings.Contains(s, "danio"):
		return AnimalZebrafish
	case strings.Contains(s, "balb") || strings.Contains(s, "icr") || strings.Contains(s, "km") || strings.Contains(s, "白"):
		return AnimalMouseWhite
	default:
		return AnimalMouseWhite
	}
}
