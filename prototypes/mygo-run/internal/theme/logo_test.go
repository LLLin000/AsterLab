package theme

import (
	"bytes"
	"image/png"
	"testing"
)

func TestBrandLogosAndICO(t *testing.T) {
	if s := LogoParsedSmall(); s == nil {
		t.Fatal("LogoParsedSmall returned nil")
	}
	if s := LogoParsedStandard(); s == nil {
		t.Fatal("LogoParsedStandard returned nil")
	}
	if s := LogoParsedOutline(); s == nil {
		t.Fatal("LogoParsedOutline returned nil")
	}
	if s := LogoParsedTriStream(false); s == nil {
		t.Fatal("LogoParsedTriStream(false) returned nil")
	}
	if s := LogoParsedTriStream(true); s == nil {
		t.Fatal("LogoParsedTriStream(true) returned nil")
	}

	appPNG := AppIconPNG()
	if len(appPNG) == 0 {
		t.Fatal("AppIconPNG returned empty bytes")
	}
	img, err := png.Decode(bytes.NewReader(appPNG))
	if err != nil {
		t.Fatalf("AppIconPNG invalid png: %v", err)
	}
	if img.Bounds().Dx() != 256 || img.Bounds().Dy() != 256 {
		t.Fatalf("unexpected AppIconPNG size: %v", img.Bounds())
	}

	wm := LogoWatermarkPNG(128)
	if len(wm) == 0 {
		t.Fatal("LogoWatermarkPNG returned empty bytes")
	}
	if _, err := png.Decode(bytes.NewReader(wm)); err != nil {
		t.Fatalf("LogoWatermarkPNG invalid png: %v", err)
	}

	ico, err := BuildMultiResICO()
	if err != nil {
		t.Fatalf("BuildMultiResICO failed: %v", err)
	}
	if len(ico) < 1000 || ico[2] != 1 || ico[3] != 0 || ico[4] != 7 {
		t.Fatalf("unexpected ICO header (len=%d)", len(ico))
	}
}
