// Command genbrand generates AsterLab desktop icon assets (multi-res .ico,
// 256px PNG tile, and optical standard/small SVGs) from internal/theme.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"asterlab-mygo-prototype/internal/theme"
)

func main() {
	outDir := filepath.Join("assets")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		panic(err)
	}

	ico, err := theme.BuildMultiResICO()
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "asterlab.ico"), ico, 0o644); err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "icon-256.png"), theme.AppIconPNG(), 0o644); err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "asterlab-symbol.svg"), []byte(theme.LogoStandardSVG), 0o644); err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "asterlab-small.svg"), []byte(theme.LogoSmallSVG), 0o644); err != nil {
		panic(err)
	}
	fmt.Printf("Generated AsterLab brand assets in %s (ico=%d bytes, png=%d bytes)\n",
		outDir, len(ico), len(theme.AppIconPNG()))
}
