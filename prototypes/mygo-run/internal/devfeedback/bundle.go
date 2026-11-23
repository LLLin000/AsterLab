// Package devfeedback implements developer-only UI visual issue annotation,
// DPI-aware screenshot red-box rendering, zoomed crop generation, workbench
// region classification, and structured feedback bundle (.zip + directory)
// export for the AsterLab developer debugging loop.
package devfeedback

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Rect is a rectangle in window DIP coordinates.
type Rect struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
	W float32 `json:"w"`
	H float32 `json:"h"`
}

// RegionHint describes the semantic workbench region where a UI issue was picked.
type RegionHint struct {
	Key        string `json:"key"`
	Label      string `json:"label"`
	SourceHint string `json:"source_hint"`
}

// SessionMeta captures window/app-level context for a feedback export bundle.
type SessionMeta struct {
	SessionID   string  `json:"session_id"`
	ExportedAt  string  `json:"exported_at"`
	AppVersion  string  `json:"app_version"`
	Commit      string  `json:"commit"`
	OS          string  `json:"os"`
	Arch        string  `json:"arch"`
	WindowW     float32 `json:"window_w_dip"`
	WindowH     float32 `json:"window_h_dip"`
	UIScale     float32 `json:"ui_scale"`
	DarkTheme   bool    `json:"dark_theme"`
	VaultRoot   string  `json:"vault_root,omitempty"`
	ActiveTab   string  `json:"active_tab,omitempty"`
	ActiveFile  string  `json:"active_file,omitempty"`
	ActiveMode  string  `json:"active_mode,omitempty"`
	PageName    string  `json:"page_name,omitempty"`
}

// Issue is a single developer-annotated UI defect / suggestion.
type Issue struct {
	ID               string     `json:"id"`
	CreatedAt        string     `json:"created_at"`
	Category         string     `json:"category"` // 样式/间距 | 文字裁切 | 交互逻辑 | 功能Bug
	Severity         string     `json:"severity"` // P1 | P2 | P3
	PickKind         string     `json:"pick_kind"` // click | drag_box
	RectDIP          Rect       `json:"rect_dip"`
	WindowW          float32    `json:"window_w_dip"`
	WindowH          float32    `json:"window_h_dip"`
	ImageW           int        `json:"image_w_px"`
	ImageH           int        `json:"image_h_px"`
	Region           RegionHint `json:"region"`
	PageName         string     `json:"page_name"`
	ActiveTab        string     `json:"active_tab,omitempty"`
	ActiveFile       string     `json:"active_file,omitempty"`
	ActiveMode       string     `json:"active_mode,omitempty"`
	RightDrawerOpen  bool       `json:"right_drawer_open"`
	DarkTheme        bool       `json:"dark_theme"`
	UIScale          float32    `json:"ui_scale"`
	InspectorSnippet string     `json:"inspector_snippet,omitempty"`
	Suggestion       string     `json:"suggestion"`
	FullShotFile     string     `json:"full_shot_file,omitempty"`
	CropShotFile     string     `json:"crop_shot_file,omitempty"`

	// Raw PNG payloads kept in memory until exported.
	FullPNG []byte `json:"-"`
	CropPNG []byte `json:"-"`
}

// Manifest is the machine-readable manifest.json written inside a feedback bundle.
type Manifest struct {
	Schema  int         `json:"schema"`
	Session SessionMeta `json:"session"`
	Issues  []Issue     `json:"issues"`
}

// DetectRegion maps a DIP window coordinate to the closest semantic AsterLab workbench zone
// along with the primary Go source file responsible for rendering that zone.
func DetectRegion(x, y, winW, winH, leftW, rightW float32, leftOpen, rightOpen bool, pageName, tabKind string) RegionHint {
	if leftOpen && leftW > 0 && x <= leftW {
		if y <= 46 {
			return RegionHint{
				Key:        "left-sidebar-header",
				Label:      "左侧栏顶部操作区",
				SourceHint: "prototypes/mygo-run/internal/shell/file_tree.go",
			}
		}
		if winH > 0 && y >= winH-72 {
			return RegionHint{
				Key:        "left-sidebar-footer",
				Label:      "左侧栏底部（储藏室 / 设置）",
				SourceHint: "prototypes/mygo-run/internal/shell/shell.go",
			}
		}
		return RegionHint{
			Key:        "left-file-tree",
			Label:      "左侧文件树 / 导航栏",
			SourceHint: "prototypes/mygo-run/internal/shell/file_tree.go",
		}
	}
	if rightOpen && rightW > 0 && winW > 0 && x >= winW-rightW {
		if y <= 48 {
			return RegionHint{
				Key:        "right-drawer-header",
				Label:      "右侧抽屉标题栏",
				SourceHint: "prototypes/mygo-run/internal/shell/right_drawer.go",
			}
		}
		return RegionHint{
			Key:        "right-companion-drawer",
			Label:      "右侧辅助抽屉（大纲 / 计算器 / 诊断）",
			SourceHint: "prototypes/mygo-run/internal/shell/right_drawer.go",
		}
	}
	if y <= 44 {
		return RegionHint{
			Key:        "top-tab-strip",
			Label:      "顶部标签页栏 / 导航条",
			SourceHint: "prototypes/mygo-run/internal/shell/tabs.go",
		}
	}
	switch tabKind {
	case "calendar":
		if y <= 104 {
			return RegionHint{
				Key:        "calendar-toolbar",
				Label:      "实验日程工具栏",
				SourceHint: "prototypes/mygo-run/internal/shell/calendar.go",
			}
		}
		return RegionHint{
			Key:        "calendar-grid",
			Label:      "实验日程视图",
			SourceHint: "prototypes/mygo-run/internal/shell/calendar.go",
		}
	case "pantry", "tools":
		return RegionHint{
			Key:        "pantry-workbench",
			Label:      "试剂耗材储藏室 / 工具箱",
			SourceHint: "prototypes/mygo-run/pantry_page.go",
		}
	case "file":
		if y <= 98 {
			return RegionHint{
				Key:        "doc-header-bar",
				Label:      "实验文档顶栏（标题 / 视图切换 / 导出）",
				SourceHint: "prototypes/mygo-run/vault_host.go",
			}
		}
		if winH > 0 && y >= winH-36 {
			return RegionHint{
				Key:        "status-bar",
				Label:      "底部状态栏",
				SourceHint: "prototypes/mygo-run/internal/shell/shell.go",
			}
		}
		return RegionHint{
			Key:        "live-asterml-editor",
			Label:      "实验文档编辑区（Live Preview / 模块卡片）",
			SourceHint: "prototypes/mygo-run/internal/asterdoc/live_markdown.go",
		}
	}
	if pageName == "tools" {
		return RegionHint{
			Key:        "pantry-workbench",
			Label:      "试剂耗材储藏室",
			SourceHint: "prototypes/mygo-run/pantry_page.go",
		}
	}
	return RegionHint{
		Key:        "main-workbench",
		Label:      "主工作区内容面板",
		SourceHint: "prototypes/mygo-run/internal/shell/shell.go",
	}
}

// AnnotateAndCrop takes a raw PNG screenshot of the window and a selection Rect in DIP,
// draws a crisp red highlight rectangle on the full frame, and produces a padded 2x zoomed
// crop focused on the selected detail so fine typography/spacing defects are immediately visible.
func AnnotateAndCrop(rawPNG []byte, rectDIP Rect, winW, winH float32) (fullAnnotated []byte, cropZoomed []byte, imgW, imgH int, err error) {
	if len(rawPNG) == 0 {
		return nil, nil, 0, 0, fmt.Errorf("empty screenshot payload")
	}
	src, err := png.Decode(bytes.NewReader(rawPNG))
	if err != nil {
		return nil, nil, 0, 0, fmt.Errorf("decode screenshot png: %w", err)
	}
	b := src.Bounds()
	imgW, imgH = b.Dx(), b.Dy()
	if imgW <= 0 || imgH <= 0 {
		return nil, nil, 0, 0, fmt.Errorf("invalid screenshot dimensions %dx%d", imgW, imgH)
	}

	// Copy into mutable RGBA image with origin (0,0).
	full := image.NewRGBA(image.Rect(0, 0, imgW, imgH))
	draw.Draw(full, full.Bounds(), src, b.Min, draw.Src)

	scaleX := float64(1)
	scaleY := float64(1)
	if winW > 0 {
		scaleX = float64(imgW) / float64(winW)
	}
	if winH > 0 {
		scaleY = float64(imgH) / float64(winH)
	}

	// Normalize DIP rect; if click-point (tiny box), expand into a visible 48x32 DIP marker.
	rx, ry, rw, rh := rectDIP.X, rectDIP.Y, rectDIP.W, rectDIP.H
	if rw < 0 {
		rx += rw
		rw = -rw
	}
	if rh < 0 {
		ry += rh
		rh = -rh
	}
	if rw < 12 && rh < 12 {
		cx := rx + rw*0.5
		cy := ry + rh*0.5
		rw, rh = 48, 32
		rx, ry = cx-rw*0.5, cy-rh*0.5
	}

	px0 := clampInt(int(math.Round(float64(rx)*scaleX)), 0, imgW-1)
	py0 := clampInt(int(math.Round(float64(ry)*scaleY)), 0, imgH-1)
	px1 := clampInt(int(math.Round(float64(rx+rw)*scaleX)), px0+1, imgW)
	py1 := clampInt(int(math.Round(float64(ry+rh)*scaleY)), py0+1, imgH)
	box := image.Rect(px0, py0, px1, py1)

	// Compute padded crop around box BEFORE drawing onto full, then draw red box on both.
	pad := clampInt(maxInt(box.Dx(), box.Dy())/2, 48, 120)
	cx0 := clampInt(box.Min.X-pad, 0, imgW-1)
	cy0 := clampInt(box.Min.Y-pad, 0, imgH-1)
	cx1 := clampInt(box.Max.X+pad, cx0+1, imgW)
	cy1 := clampInt(box.Max.Y+pad, cy0+1, imgH)
	cropRect := image.Rect(cx0, cy0, cx1, cy1)

	crop := image.NewRGBA(image.Rect(0, 0, cropRect.Dx(), cropRect.Dy()))
	draw.Draw(crop, crop.Bounds(), full, cropRect.Min, draw.Src)

	// Draw crisp red highlight box on both full screenshot and crop.
	borderThick := maxInt(2, int(math.Round(2.5*math.Max(scaleX, 1))))
	drawHighlightBox(full, box, borderThick)
	relBox := box.Sub(cropRect.Min)
	drawHighlightBox(crop, relBox, maxInt(2, borderThick-1))

	// Zoom crop 2x when small so subtle typography/border glitches stand out clearly.
	var finalCrop image.Image = crop
	if crop.Bounds().Dx() <= 640 && crop.Bounds().Dy() <= 480 {
		finalCrop = scaleNearest2x(crop)
	}

	var fullBuf, cropBuf bytes.Buffer
	if err := png.Encode(&fullBuf, full); err != nil {
		return nil, nil, imgW, imgH, fmt.Errorf("encode annotated full png: %w", err)
	}
	if err := png.Encode(&cropBuf, finalCrop); err != nil {
		return nil, nil, imgW, imgH, fmt.Errorf("encode zoomed crop png: %w", err)
	}
	return fullBuf.Bytes(), cropBuf.Bytes(), imgW, imgH, nil
}

func drawHighlightBox(img *image.RGBA, r image.Rectangle, thick int) {
	b := img.Bounds()
	r = r.Intersect(b)
	if r.Empty() {
		return
	}
	// Soft translucent red interior tint so underlying text stays 100% readable.
	tint := color.NRGBA{R: 239, G: 68, B: 68, A: 26}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			blendPixel(img, x, y, tint)
		}
	}
	// Crisp solid red outer border (#EF4444) + subtle white 1px outer halo for dark mode contrast.
	red := color.RGBA{R: 239, G: 68, B: 68, A: 255}
	halo := color.RGBA{R: 255, G: 255, B: 255, A: 210}
	drawRectBorder(img, image.Rect(r.Min.X-1, r.Min.Y-1, r.Max.X+1, r.Max.Y+1), 1, halo)
	drawRectBorder(img, r, thick, red)
}

func drawRectBorder(img *image.RGBA, r image.Rectangle, thick int, col color.RGBA) {
	b := img.Bounds()
	for t := 0; t < thick; t++ {
		x0, y0 := r.Min.X+t, r.Min.Y+t
		x1, y1 := r.Max.X-1-t, r.Max.Y-1-t
		if x0 > x1 || y0 > y1 {
			break
		}
		for x := x0; x <= x1; x++ {
			if x >= b.Min.X && x < b.Max.X {
				if y0 >= b.Min.Y && y0 < b.Max.Y {
					img.SetRGBA(x, y0, col)
				}
				if y1 >= b.Min.Y && y1 < b.Max.Y {
					img.SetRGBA(x, y1, col)
				}
			}
		}
		for y := y0; y <= y1; y++ {
			if y >= b.Min.Y && y < b.Max.Y {
				if x0 >= b.Min.X && x0 < b.Max.X {
					img.SetRGBA(x0, y, col)
				}
				if x1 >= b.Min.X && x1 < b.Max.X {
					img.SetRGBA(x1, y, col)
				}
			}
		}
	}
}

func blendPixel(img *image.RGBA, x, y int, src color.NRGBA) {
	dst := img.RGBAAt(x, y)
	a := uint32(src.A)
	inv := 255 - a
	img.SetRGBA(x, y, color.RGBA{
		R: uint8((uint32(src.R)*a + uint32(dst.R)*inv) / 255),
		G: uint8((uint32(src.G)*a + uint32(dst.G)*inv) / 255),
		B: uint8((uint32(src.B)*a + uint32(dst.B)*inv) / 255),
		A: 255,
	})
}

func scaleNearest2x(src *image.RGBA) *image.RGBA {
	sb := src.Bounds()
	w, h := sb.Dx(), sb.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w*2, h*2))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := src.RGBAAt(sb.Min.X+x, sb.Min.Y+y)
			dx, dy := x*2, y*2
			dst.SetRGBA(dx, dy, c)
			dst.SetRGBA(dx+1, dy, c)
			dst.SetRGBA(dx, dy+1, c)
			dst.SetRGBA(dx+1, dy+1, c)
		}
	}
	return dst
}

// RenderMarkdown generates an AI-agent-friendly feedback.md summarizing every recorded UI issue.
func RenderMarkdown(session SessionMeta, issues []Issue) string {
	var b strings.Builder
	b.WriteString("# AsterLab UI 调试反馈包\n\n")
	b.WriteString("## 环境信息\n\n")
	fmt.Fprintf(&b, "- **Session**: `%s`\n", session.SessionID)
	fmt.Fprintf(&b, "- **导出时间**: `%s`\n", session.ExportedAt)
	fmt.Fprintf(&b, "- **版本 / Commit**: `%s` (`%s`)\n", fallback(session.AppVersion, "dev"), fallback(session.Commit, "local"))
	fmt.Fprintf(&b, "- **平台**: `%s/%s` · 窗口 `%.0f×%.0f DIP` · 缩放 `%.2fx` · 主题 `%s`\n",
		session.OS, session.Arch, session.WindowW, session.WindowH, session.UIScale, themeName(session.DarkTheme))
	if session.ActiveFile != "" {
		fmt.Fprintf(&b, "- **当前文档**: `%s` (`%s`)\n", session.ActiveFile, fallback(session.ActiveMode, "live"))
	}
	b.WriteString("\n## 问题清单总览\n\n")
	b.WriteString("| ID | 严重度 | 分类 | 区域 | 坐标 (DIP) | 修改建议 |\n")
	b.WriteString("|---|---|---|---|---|---|\n")
	for _, it := range issues {
		summary := strings.ReplaceAll(strings.TrimSpace(it.Suggestion), "\n", " ")
		if summary == "" {
			summary = "（未填写建议）"
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s | %s | `(%.0f, %.0f, %.0f×%.0f)` | %s |\n",
			it.ID, fallback(it.Severity, "P2"), fallback(it.Category, "样式/间距"),
			fallback(it.Region.Label, it.Region.Key),
			it.RectDIP.X, it.RectDIP.Y, it.RectDIP.W, it.RectDIP.H,
			escapePipe(summary))
	}
	b.WriteString("\n---\n\n")
	for _, it := range issues {
		fmt.Fprintf(&b, "## %s · [%s] %s — %s\n\n",
			it.ID, fallback(it.Severity, "P2"), fallback(it.Category, "样式/间距"),
			fallback(it.Region.Label, it.Region.Key))
		fmt.Fprintf(&b, "- **拾取方式**: `%s` · **坐标 (DIP)**: `x=%.0f, y=%.0f, w=%.0f, h=%.0f` (窗口 `%.0f×%.0f`, 截图 `%d×%d px`)\n",
			fallback(it.PickKind, "click"), it.RectDIP.X, it.RectDIP.Y, it.RectDIP.W, it.RectDIP.H,
			it.WindowW, it.WindowH, it.ImageW, it.ImageH)
		if it.Region.SourceHint != "" {
			fmt.Fprintf(&b, "- **建议定位源码**: `%s` (`%s`)\n", it.Region.SourceHint, it.Region.Key)
		}
		fmt.Fprintf(&b, "- **页面上下文**: page=`%s`, tab=`%s`, file=`%s`, mode=`%s`, rightDrawer=`%v`, theme=`%s`, scale=`%.2fx`\n",
			fallback(it.PageName, "-"), fallback(it.ActiveTab, "-"), fallback(it.ActiveFile, "-"),
			fallback(it.ActiveMode, "-"), it.RightDrawerOpen, themeName(it.DarkTheme), it.UIScale)
		fmt.Fprintf(&b, "- **修改建议**:\n\n> %s\n\n", strings.ReplaceAll(fallback(strings.TrimSpace(it.Suggestion), "（未填写）"), "\n", "\n> "))
		if strings.TrimSpace(it.InspectorSnippet) != "" {
			b.WriteString("- **MyGo Inspector (F12) 精确元素/源码定位**:\n\n```text\n")
			b.WriteString(strings.TrimSpace(it.InspectorSnippet))
			b.WriteString("\n```\n\n")
		}
		if it.CropShotFile != "" {
			fmt.Fprintf(&b, "### 局部放大截图\n\n![%s crop](%s)\n\n", it.ID, it.CropShotFile)
		}
		if it.FullShotFile != "" {
			fmt.Fprintf(&b, "### 全窗红框定位\n\n![%s full](%s)\n\n", it.ID, it.FullShotFile)
		}
	}
	return b.String()
}

// ExportSession writes both an uncompressed timestamped feedback folder and a matching .zip archive
// containing feedback.md, manifest.json, shots/*.png, and logs/recent.log.
func ExportSession(outBaseDir string, session SessionMeta, issues []Issue, recentLogs []string) (folderPath, zipPath string, err error) {
	if len(issues) == 0 {
		return "", "", fmt.Errorf("no UI issues recorded yet")
	}
	now := time.Now()
	if session.SessionID == "" {
		session.SessionID = now.Format("20060102-150405")
	}
	if session.ExportedAt == "" {
		session.ExportedAt = now.Format(time.RFC3339)
	}
	if err := os.MkdirAll(outBaseDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create feedback output dir: %w", err)
	}

	bundleName := "asterlab-ui-feedback-" + session.SessionID
	folderPath = filepath.Join(outBaseDir, bundleName)
	shotsDir := filepath.Join(folderPath, "shots")
	logsDir := filepath.Join(folderPath, "logs")
	if err := os.MkdirAll(shotsDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create shots dir: %w", err)
	}
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create logs dir: %w", err)
	}

	outIssues := make([]Issue, len(issues))
	for i, it := range issues {
		if it.ID == "" {
			it.ID = fmt.Sprintf("UI-%02d", i+1)
		}
		slug := strings.ToLower(strings.ReplaceAll(it.ID, " ", "-"))
		if len(it.FullPNG) > 0 {
			rel := fmt.Sprintf("shots/%s-full.png", slug)
			if err := os.WriteFile(filepath.Join(folderPath, filepath.FromSlash(rel)), it.FullPNG, 0o644); err != nil {
				return "", "", fmt.Errorf("write %s: %w", rel, err)
			}
			it.FullShotFile = rel
		}
		if len(it.CropPNG) > 0 {
			rel := fmt.Sprintf("shots/%s-crop.png", slug)
			if err := os.WriteFile(filepath.Join(folderPath, filepath.FromSlash(rel)), it.CropPNG, 0o644); err != nil {
				return "", "", fmt.Errorf("write %s: %w", rel, err)
			}
			it.CropShotFile = rel
		}
		outIssues[i] = it
	}

	md := RenderMarkdown(session, outIssues)
	if err := os.WriteFile(filepath.Join(folderPath, "feedback.md"), []byte(md), 0o644); err != nil {
		return "", "", fmt.Errorf("write feedback.md: %w", err)
	}

	manifest := Manifest{Schema: 1, Session: session, Issues: outIssues}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", "", fmt.Errorf("marshal manifest.json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(folderPath, "manifest.json"), manifestBytes, 0o644); err != nil {
		return "", "", fmt.Errorf("write manifest.json: %w", err)
	}

	logContent := strings.Join(recentLogs, "\n")
	if logContent != "" && !strings.HasSuffix(logContent, "\n") {
		logContent += "\n"
	}
	if err := os.WriteFile(filepath.Join(logsDir, "recent.log"), []byte(logContent), 0o644); err != nil {
		return "", "", fmt.Errorf("write recent.log: %w", err)
	}

	zipPath = folderPath + ".zip"
	if err := zipDirectory(folderPath, zipPath); err != nil {
		return folderPath, "", fmt.Errorf("create zip archive: %w", err)
	}
	return folderPath, zipPath, nil
}

// ResolveWritableOutputDir probes candidate directories in order by creating each
// directory and verifying actual file creation/removal, avoiding paths blocked by
// OS/DLP/Desktop folder protection (e.g. Access is denied on C:\Users\...\Desktop).
func ResolveWritableOutputDir(candidates ...string) (string, error) {
	var lastErr error
	for _, raw := range candidates {
		dir := strings.TrimSpace(raw)
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			lastErr = err
			continue
		}
		probe := filepath.Join(dir, fmt.Sprintf(".write-probe-%d", time.Now().UnixNano()))
		if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
			lastErr = err
			continue
		}
		_ = os.Remove(probe)
		return dir, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no candidate output directories provided")
	}
	return "", fmt.Errorf("no writable feedback directory found: %w", lastErr)
}

const autosaveDirName = "autosave-draft"

// SaveDraft persists the current in-memory issue queue (including PNG screenshots)
// into <baseDir>/autosave-draft so notes are never lost if the dev app restarts.
func SaveDraft(baseDir string, issues []Issue) error {
	if strings.TrimSpace(baseDir) == "" {
		return fmt.Errorf("empty draft base dir")
	}
	draftDir := filepath.Join(baseDir, autosaveDirName)
	shotsDir := filepath.Join(draftDir, "shots")
	if len(issues) == 0 {
		return ClearDraft(baseDir)
	}
	if err := os.MkdirAll(shotsDir, 0o755); err != nil {
		return fmt.Errorf("create draft shots dir: %w", err)
	}
	saved := make([]Issue, len(issues))
	for i, it := range issues {
		slug := strings.ToLower(strings.ReplaceAll(fallback(it.ID, fmt.Sprintf("UI-%02d", i+1)), " ", "-"))
		if len(it.FullPNG) > 0 {
			rel := fmt.Sprintf("shots/%s-full.png", slug)
			if err := os.WriteFile(filepath.Join(draftDir, filepath.FromSlash(rel)), it.FullPNG, 0o644); err != nil {
				return fmt.Errorf("write draft %s: %w", rel, err)
			}
			it.FullShotFile = rel
		}
		if len(it.CropPNG) > 0 {
			rel := fmt.Sprintf("shots/%s-crop.png", slug)
			if err := os.WriteFile(filepath.Join(draftDir, filepath.FromSlash(rel)), it.CropPNG, 0o644); err != nil {
				return fmt.Errorf("write draft %s: %w", rel, err)
			}
			it.CropShotFile = rel
		}
		saved[i] = it
	}
	raw, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal draft.json: %w", err)
	}
	return os.WriteFile(filepath.Join(draftDir, "draft.json"), raw, 0o644)
}

// LoadDraft restores any previously auto-saved UI feedback queue from <baseDir>/autosave-draft.
func LoadDraft(baseDir string) ([]Issue, error) {
	if strings.TrimSpace(baseDir) == "" {
		return nil, nil
	}
	draftDir := filepath.Join(baseDir, autosaveDirName)
	raw, err := os.ReadFile(filepath.Join(draftDir, "draft.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var issues []Issue
	if err := json.Unmarshal(raw, &issues); err != nil {
		return nil, err
	}
	for i := range issues {
		if issues[i].FullShotFile != "" {
			if b, rErr := os.ReadFile(filepath.Join(draftDir, filepath.FromSlash(issues[i].FullShotFile))); rErr == nil {
				issues[i].FullPNG = b
			}
		}
		if issues[i].CropShotFile != "" {
			if b, rErr := os.ReadFile(filepath.Join(draftDir, filepath.FromSlash(issues[i].CropShotFile))); rErr == nil {
				issues[i].CropPNG = b
			}
		}
	}
	return issues, nil
}

// ClearDraft removes the auto-saved draft directory once exported or explicitly cleared.
func ClearDraft(baseDir string) error {
	if strings.TrimSpace(baseDir) == "" {
		return nil
	}
	return os.RemoveAll(filepath.Join(baseDir, autosaveDirName))
}

func zipDirectory(srcDir, dstZip string) error {
	f, err := os.Create(dstZip)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	baseName := filepath.Base(srcDir)
	return filepath.WalkDir(srcDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return walkErr
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		w, err := zw.Create(filepath.ToSlash(filepath.Join(baseName, rel)))
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	})
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func fallback(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

func themeName(dark bool) string {
	if dark {
		return "dark"
	}
	return "light"
}

func escapePipe(s string) string {
	return strings.ReplaceAll(s, "|", `\|`)
}
