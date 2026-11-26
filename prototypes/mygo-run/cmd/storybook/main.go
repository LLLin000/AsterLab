// Command storybook launches AsterLab's standalone UI Component Gallery outside the main app.
package main

import (
	"flag"
	"log"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/storybook"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/uikit/kit"
)

type storybookApp struct {
	state    *kit.GalleryState
	sections []kit.StorySection
}

func newStorybookApp(mobile, dark bool, section string) *storybookApp {
	active := section
	if active == "" {
		if mobile {
			active = "mobile-ios-design"
		} else {
			active = "all"
		}
	}
	themeOverride := "follow"
	if dark {
		themeOverride = "dark"
	}
	return &storybookApp{
		state: &kit.GalleryState{
			ActiveSection: active,
			MobilePreview: mobile,
			ThemeOverride: themeOverride,
			SampleNum:     12,
			ScrubVal:      22.5,
			SelectedIdx:   1,
		},
		sections: storybook.BuildAllSections(),
	}
}

func (a *storybookApp) View(c *ui.Context) {
	c.SetTheme(theme.Window(c))
	kit.RenderComponentGallery(c, a.state, a.sections...)
}

func main() {
	mobileFlag := flag.Bool("mobile", false, "start directly on Section 8 (Mobile 402dp iOS Spec) with 402dp preview")
	darkFlag := flag.Bool("dark", false, "start in dark theme")
	sectionFlag := flag.String("section", "", "initial gallery section ID (defaults to all, or mobile-ios-design with -mobile)")
	flag.Parse()

	app := newStorybookApp(*mobileFlag, *darkFlag, *sectionFlag)
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
			Title:     "AsterLab UI 组件画廊 (Storybook)",
			Width:     1360,
			Height:    880,
			MinWidth:  402,
			MinHeight: 640,
			Content:   ui.View(app.View),
		})
	})
	_ = time.Second
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
