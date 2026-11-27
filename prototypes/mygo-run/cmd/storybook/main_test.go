package main

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/theme"
)

func TestStandaloneStorybookRendersAllSections(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mobile bool
		dark   bool
		w, h   int
	}{
		{"desktop_1360x880_light", false, false, 1360, 880},
		{"mobile_402x874_dark", true, true, 402, 874},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := newStorybookApp(tc.mobile, tc.dark, "")
			tt := ui.NewTester(func(c *ui.Context) {
				if tc.dark {
					c.SetTheme(theme.DarkTheme())
				} else {
					c.SetTheme(theme.LightTheme())
				}
				app.View(c)
			}, tc.w, tc.h)
			tt.Frame()
			if !tt.HasText("AsterLab 统一 UI 组件画廊") {
				t.Fatalf("missing gallery header, texts=%v", tt.Texts())
			}
			if tc.mobile {
				if !tt.HasText("8A. 402dp 移动工作台壳层、顶/底 SafeArea 避让与手势闭环") {
					t.Fatalf("missing Section 8A on -mobile view, texts=%v", tt.Texts())
				}
			} else {
				for _, heading := range []string{
					"1A. 全局色彩色板与状态 Token",
					"2A. 基础交互控件 & 6 大 UI 逻辑引擎",
					"3A. 空间实验载体（孔板 · 平皿 · 管架 · 流程节点）",
					"3D. 暗房灯箱与印样画廊",
					"4A. 顶栏标签页、分屏与模式头",
					"5A. AsterDoc 单真源文档组件实战预览",
					"6A. 动物房合规卡片与边界压测态",
					"7A. 宽屏编辑态：极简 Tufte 右侧边注（默认展开 · Y 轴防碰撞）+ @ 行内药丸",
					"8A. 402dp 移动工作台壳层、顶/底 SafeArea 避让与手势闭环",
					"8B. 全屏移动页面规范（两级设置页 & 全屏表单导航栏）",
				} {
					if !tt.HasText(heading) {
						t.Fatalf("standalone gallery missing %q, texts=%v", heading, tt.Texts())
					}
				}
			}
		})
	}
}

func TestExportStorybookScreenshotsWhenRequested(t *testing.T) {
	dir := os.Getenv("ASTERLAB_STORYBOOK_SHOT_DIR")
	if dir == "" {
		return
	}
	saveShot := func(name string, tt *ui.Tester) {
		t.Helper()
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}

	// 1. Desktop Light 1360x880 (Section 1 + full overview)
	{
		app := newStorybookApp(false, false, "")
		tt := ui.NewTesterWith(func(c *ui.Context) {
			c.SetTheme(theme.LightTheme())
			app.View(c)
		}, 1360, 880, ui.TesterOptions{Dark: false})
		tt.Frame()
		tt.Advance(250 * time.Millisecond)
		tt.Frame()
		saveShot("gallery_desktop_light_1360x880.png", tt)
	}

	// 2. Desktop Dark 1360x880 (focused on darkroom-contact-sheet so contact sheet + elevation are crisp)
	{
		app := newStorybookApp(false, true, "darkroom-contact-sheet")
		tt := ui.NewTesterWith(func(c *ui.Context) {
			c.SetTheme(theme.DarkTheme())
			app.View(c)
		}, 1360, 880, ui.TesterOptions{Dark: true})
		tt.Frame()
		tt.Advance(250 * time.Millisecond)
		tt.Frame()
		saveShot("gallery_desktop_dark_1360x880.png", tt)
	}

	// 3. Mobile Dark 402x874 (3-col Contact Sheet + responsive header)
	{
		app := newStorybookApp(false, true, "darkroom-contact-sheet")
		tt := ui.NewTesterWith(func(c *ui.Context) {
			c.SetTheme(theme.DarkTheme())
			app.View(c)
		}, 402, 874, ui.TesterOptions{Dark: true})
		tt.Frame()
		tt.Advance(250 * time.Millisecond)
		tt.Frame()
		saveShot("gallery_mobile_dark_402x874.png", tt)

		// 4. Mobile Lightbox 402x874 (tap first tile to enter DarkroomLightbox)
		if err := tt.Click("HUVEC · DAPI+FITC 共定位"); err != nil {
			t.Fatalf("click specimen tile to open lightbox: %v", err)
		}
		tt.Frame()
		tt.Advance(300 * time.Millisecond)
		tt.Frame()
		if !tt.HasText("插入当前实验") {
			t.Fatalf("expected DarkroomLightbox open in screenshot, texts=%v", tt.Texts())
		}
		saveShot("gallery_lightbox_402x874.png", tt)
	}
}
