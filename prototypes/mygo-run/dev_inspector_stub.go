//go:build !devdebug

package main

import (
	"asterlab-mygo-prototype/internal/shell"

	"github.com/egoist/mygo/ui"
)

// devInspectorState is zero-sized in production / user builds so no developer
// UI inspection code, shortcuts, or feedback UI are compiled into shipped releases.
type devInspectorState struct{}

func (a *app) renderDevInspectorOverlay(_ *ui.Context) {}

func (a *app) devInspectorPaletteCommands() []shell.PaletteCommand { return nil }

func (a *app) resetDevFeedbackDirCache() {}
