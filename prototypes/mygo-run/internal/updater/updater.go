package updater

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Build-time injected metadata (set via -ldflags "-X ...").
var (
	Version    = "0.1.0-dev"
	Commit     = "dev"
	Repo       = "LLLin000/AsterLab"
	APIBaseURL = "https://api.github.com"
)

// Status describes the lifecycle state of in-app update checking and installation.
type Status string

const (
	StatusIdle            Status = ""
	StatusChecking        Status = "checking"
	StatusUpToDate        Status = "up_to_date"
	StatusAvailable       Status = "available"
	StatusDownloading     Status = "downloading"
	StatusReadyToRestart  Status = "restarting"
	StatusError           Status = "error"
)

// ReleaseAsset describes one downloadable binary or checksum file attached to a release.
type ReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	ContentType        string `json:"content_type"`
}

// ReleaseInfo models a GitHub release payload plus matched Windows distribution assets.
type ReleaseInfo struct {
	TagName     string         `json:"tag_name"`
	Name        string         `json:"name"`
	Body        string         `json:"body"`
	HTMLURL     string         `json:"html_url"`
	Prerelease  bool           `json:"prerelease"`
	Draft       bool           `json:"draft"`
	PublishedAt string         `json:"published_at"`
	Assets      []ReleaseAsset `json:"assets"`

	ZipURL    string `json:"-"`
	ZipName   string `json:"-"`
	SetupURL  string `json:"-"`
	SetupName string `json:"-"`
	SumsURL   string `json:"-"`
}

// CheckResult is returned by CheckForUpdate.
type CheckResult struct {
	CurrentVersion string
	Latest         *ReleaseInfo
	HasUpdate      bool
	Summary        string
}

type parsedSemver struct {
	major int
	minor int
	patch int
	pre   []string
	valid bool
}

// NormalizeVersion strips leading 'v', build metadata ('+...'), and packaging suffixes like '-unsigned'.
func NormalizeVersion(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "v")
	s = strings.TrimPrefix(s, "V")
	if idx := strings.IndexByte(s, '+'); idx >= 0 {
		s = s[:idx]
	}
	s = strings.TrimSuffix(s, "-unsigned")
	s = strings.TrimSuffix(s, ".unsigned")
	return strings.TrimSpace(s)
}

func parseSemver(raw string) parsedSemver {
	norm := NormalizeVersion(raw)
	if norm == "" {
		return parsedSemver{}
	}
	core := norm
	var preParts []string
	if idx := strings.IndexByte(norm, '-'); idx >= 0 {
		core = norm[:idx]
		preStr := norm[idx+1:]
		if preStr != "" {
			preParts = strings.Split(preStr, ".")
		}
	}
	nums := strings.Split(core, ".")
	if len(nums) < 1 || len(nums) > 3 {
		return parsedSemver{}
	}
	var vals [3]int
	for i, part := range nums {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return parsedSemver{}
		}
		vals[i] = n
	}
	return parsedSemver{
		major: vals[0],
		minor: vals[1],
		patch: vals[2],
		pre:   preParts,
		valid: true,
	}
}

func comparePrerelease(a, b []string) int {
	switch {
	case len(a) == 0 && len(b) == 0:
		return 0
	case len(a) == 0:
		return 1 // release > prerelease
	case len(b) == 0:
		return -1
	}
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] == b[i] {
			continue
		}
		an, aErr := strconv.Atoi(a[i])
		bn, bErr := strconv.Atoi(b[i])
		switch {
		case aErr == nil && bErr == nil:
			if an < bn {
				return -1
			}
			return 1
		case aErr == nil:
			return -1 // numeric identifiers have lower precedence than non-numeric
		case bErr == nil:
			return 1
		default:
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return 0
}

// CompareVersions compares two version strings (-1 if a < b, 0 if equal, +1 if a > b).
func CompareVersions(a, b string) int {
	pa, pb := parseSemver(a), parseSemver(b)
	if !pa.valid || !pb.valid {
		na, nb := NormalizeVersion(a), NormalizeVersion(b)
		return strings.Compare(na, nb)
	}
	if pa.major != pb.major {
		if pa.major < pb.major {
			return -1
		}
		return 1
	}
	if pa.minor != pb.minor {
		if pa.minor < pb.minor {
			return -1
		}
		return 1
	}
	if pa.patch != pb.patch {
		if pa.patch < pb.patch {
			return -1
		}
		return 1
	}
	return comparePrerelease(pa.pre, pb.pre)
}

func populateMatchedAssets(rel *ReleaseInfo) {
	if rel == nil {
		return
	}
	for _, a := range rel.Assets {
		lower := strings.ToLower(a.Name)
		switch {
		case lower == "sha256sums.txt":
			rel.SumsURL = a.BrowserDownloadURL
		case strings.Contains(lower, "win-x64") && strings.HasSuffix(lower, ".zip"):
			rel.ZipURL = a.BrowserDownloadURL
			rel.ZipName = a.Name
		case strings.Contains(lower, "win-x64") && strings.HasSuffix(lower, "setup.exe"):
			rel.SetupURL = a.BrowserDownloadURL
			rel.SetupName = a.Name
		}
	}
}

func validateDownloadURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return err
	}
	if u.Scheme == "https" {
		return nil
	}
	host := strings.ToLower(u.Hostname())
	if u.Scheme == "http" && (host == "127.0.0.1" || host == "localhost" || host == "::1") {
		return nil
	}
	return fmt.Errorf("不安全的更新下载地址协议：%s", u.Scheme)
}

// CheckForUpdate queries GitHub Releases and selects the newest applicable release.
func CheckForUpdate(ctx context.Context, currentVersion string, includePrerelease bool) (CheckResult, error) {
	if strings.TrimSpace(currentVersion) == "" {
		currentVersion = Version
	}
	res := CheckResult{CurrentVersion: currentVersion}
	base := strings.TrimRight(APIBaseURL, "/")
	endpoint := fmt.Sprintf("%s/repos/%s/releases?per_page=10", base, Repo)
	if err := validateDownloadURL(endpoint); err != nil {
		return res, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return res, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "AsterLab-Desktop/"+NormalizeVersion(currentVersion))

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return res, fmt.Errorf("检查更新网络失败：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return res, fmt.Errorf("获取版本信息失败 (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var releases []ReleaseInfo
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&releases); err != nil {
		return res, fmt.Errorf("解析版本信息失败：%w", err)
	}

	var best *ReleaseInfo
	for i := range releases {
		r := releases[i]
		if r.Draft {
			continue
		}
		if r.Prerelease && !includePrerelease {
			continue
		}
		if best == nil || CompareVersions(r.TagName, best.TagName) > 0 {
			copyR := r
			best = &copyR
		}
	}
	if best == nil {
		res.Summary = "未找到已发布的客户端版本"
		return res, nil
	}
	populateMatchedAssets(best)
	res.Latest = best
	res.HasUpdate = CompareVersions(best.TagName, currentVersion) > 0
	if res.HasUpdate {
		res.Summary = fmt.Sprintf("发现新版本 %s", best.TagName)
	} else {
		res.Summary = fmt.Sprintf("已是最新版本 (%s)", best.TagName)
	}
	return res, nil
}

// ParseSHA256Sums parses SHA256SUMS.txt lines like "<HASH>  <filename> (<len> bytes)".
func ParseSHA256Sums(raw string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(strings.TrimSpace(strings.TrimPrefix(line, "\ufeff")))
		if len(fields) < 2 {
			continue
		}
		hash := strings.ToLower(strings.TrimSpace(fields[0]))
		if len(hash) != 64 {
			continue
		}
		if _, err := hex.DecodeString(hash); err != nil {
			continue
		}
		name := strings.TrimPrefix(fields[1], "*")
		out[filepath.Base(name)] = hash
	}
	return out
}

func downloadToFile(ctx context.Context, rawURL, dstPath string) (string, error) {
	if err := validateDownloadURL(rawURL); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "AsterLab-Desktop/"+NormalizeVersion(Version))
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("下载失败 HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(dstPath)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, 512<<20))
	closeErr := f.Close()
	if copyErr != nil {
		return "", copyErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// DownloadAndVerifyAsset downloads a release asset into stageDir and verifies SHA256 when sumsURL is available.
func DownloadAndVerifyAsset(ctx context.Context, assetURL, sumsURL, fileName, stageDir string) (string, error) {
	if assetURL == "" || fileName == "" {
		return "", fmt.Errorf("发布版本缺少可下载文件")
	}
	if err := os.MkdirAll(stageDir, 0o755); err != nil {
		return "", err
	}
	targetPath := filepath.Join(stageDir, filepath.Base(fileName))
	gotHash, err := downloadToFile(ctx, assetURL, targetPath)
	if err != nil {
		return "", err
	}
	if sumsURL != "" {
		sumsPath := filepath.Join(stageDir, "SHA256SUMS.txt")
		if _, err := downloadToFile(ctx, sumsURL, sumsPath); err != nil {
			return "", fmt.Errorf("下载校验文件 SHA256SUMS.txt 失败：%w", err)
		}
		rawSums, err := os.ReadFile(sumsPath)
		if err != nil {
			return "", err
		}
		sums := ParseSHA256Sums(string(rawSums))
		if expected, ok := sums[filepath.Base(fileName)]; ok && !strings.EqualFold(expected, gotHash) {
			_ = os.Remove(targetPath)
			return "", fmt.Errorf("安装包 SHA256 校验失败：期望 %s，实际 %s", expected, gotHash)
		}
	}
	return targetPath, nil
}

func extractZipSafely(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()
	cleanDest := filepath.Clean(destDir)
	if err := os.MkdirAll(cleanDest, 0o755); err != nil {
		return err
	}
	for _, f := range r.File {
		rel := strings.TrimLeft(filepath.FromSlash(f.Name), string(os.PathSeparator))
		if rel == "" {
			continue
		}
		// Strip top-level package folder (AsterLab-MyGo-<ver>-win-x64/...) when present.
		parts := strings.Split(rel, string(os.PathSeparator))
		if len(parts) > 1 && strings.HasPrefix(strings.ToLower(parts[0]), "asterlab-mygo-") {
			rel = filepath.Join(parts[1:]...)
		}
		if rel == "" || rel == "." {
			continue
		}
		outPath := filepath.Join(cleanDest, rel)
		relCheck, relErr := filepath.Rel(cleanDest, outPath)
		if relErr != nil || strings.HasPrefix(relCheck, "..") {
			return fmt.Errorf("压缩包包含非法路径：%s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(outPath, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		outFile, err := os.Create(outPath)
		if err != nil {
			rc.Close()
			return err
		}
		_, copyErr := io.Copy(outFile, io.LimitReader(rc, 256<<20))
		rc.Close()
		closeErr := outFile.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func canWriteDir(dir string) bool {
	probe := filepath.Join(dir, fmt.Sprintf(".asterlab-write-%d.tmp", time.Now().UnixNano()))
	f, err := os.Create(probe)
	if err != nil {
		return false
	}
	_ = f.Close()
	_ = os.Remove(probe)
	return true
}

func copyRegularFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(dst)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(dst)
		return closeErr
	}
	return nil
}

func cleanupOldUpdatesDir(dir string) {
	if dir == "" {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		name := ent.Name()
		if strings.Contains(name, ".old") || strings.Contains(name, ".new.") {
			p := filepath.Join(dir, name)
			for i := 0; i < 3; i++ {
				if err := os.Remove(p); err == nil || os.IsNotExist(err) {
					break
				}
				time.Sleep(80 * time.Millisecond)
			}
		}
	}
}

// CleanupOldUpdates removes leftover .old / .new staging files left behind by a prior
// Windows in-place self-update or installer replacement. Call early in main().
func CleanupOldUpdates() {
	exePath, err := os.Executable()
	if err != nil || exePath == "" {
		return
	}
	cleanupOldUpdatesDir(filepath.Dir(exePath))
}

func validatePEBinary(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var hdr [2]byte
	if _, err := io.ReadFull(f, hdr[:]); err != nil {
		return err
	}
	if hdr[0] != 'M' || hdr[1] != 'Z' {
		return fmt.Errorf("更新文件 %s 不是有效的 Windows 可执行程序", filepath.Base(path))
	}
	return nil
}

// applyDirectorySwap follows go-selfupdate's two-phase Windows atomic replacement:
// stage as .new.<ts>, rename active files (even running EXEs) to .old.<ts>, rename .new -> target,
// with full rollback if any step fails.
func applyDirectorySwap(unpackedDir, installDir string) error {
	entries, err := os.ReadDir(unpackedDir)
	if err != nil {
		return err
	}
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	type swapped struct {
		dst    string
		newTmp string
		oldBak string
		hadOld bool
	}
	var plan []swapped
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		src := filepath.Join(unpackedDir, ent.Name())
		if strings.HasSuffix(strings.ToLower(ent.Name()), ".exe") {
			if err := validatePEBinary(src); err != nil {
				return err
			}
		}
		dst := filepath.Join(installDir, ent.Name())
		newTmp := dst + ".new." + stamp
		if err := copyRegularFile(src, newTmp); err != nil {
			for _, p := range plan {
				_ = os.Remove(p.newTmp)
			}
			return fmt.Errorf("写入暂存更新文件 %s 失败：%w", ent.Name(), err)
		}
		plan = append(plan, swapped{
			dst:    dst,
			newTmp: newTmp,
			oldBak: dst + ".old." + stamp,
		})
	}
	if len(plan) == 0 {
		return fmt.Errorf("更新包内未包含可替换文件")
	}

	rollback := func(upto int) {
		for i := upto; i >= 0; i-- {
			p := plan[i]
			_ = os.Remove(p.dst)
			if p.hadOld {
				_ = os.Rename(p.oldBak, p.dst)
			}
		}
		for _, p := range plan {
			_ = os.Remove(p.newTmp)
		}
	}

	for i := range plan {
		p := &plan[i]
		if _, err := os.Stat(p.dst); err == nil {
			var renameErr error
			for attempt := 0; attempt < 5; attempt++ {
				renameErr = os.Rename(p.dst, p.oldBak)
				if renameErr == nil {
					break
				}
				time.Sleep(150 * time.Millisecond)
			}
			if renameErr != nil {
				rollback(i - 1)
				return fmt.Errorf("无法重命名旧文件 %s：%w", filepath.Base(p.dst), renameErr)
			}
			p.hadOld = true
		}
		if err := os.Rename(p.newTmp, p.dst); err != nil {
			if p.hadOld {
				_ = os.Rename(p.oldBak, p.dst)
			}
			rollback(i - 1)
			return fmt.Errorf("替换更新文件 %s 失败：%w", filepath.Base(p.dst), err)
		}
	}
	return nil
}

// InstallAndScheduleRestart downloads and verifies the release archive on Windows, atomically swaps
// the executables in-place inside the same folder (go-selfupdate .old pattern), and schedules a clean
// detached relaunch after the current instance releases its single-instance lock.
func InstallAndScheduleRestart(ctx context.Context, rel *ReleaseInfo, exePath string, pid int) (string, error) {
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("当前系统请前往 GitHub Release 页面下载更新包")
	}
	if rel == nil {
		return "", fmt.Errorf("未选择可更新版本")
	}
	stageDir, err := os.MkdirTemp("", "asterlab-update-*")
	if err != nil {
		return "", err
	}
	installDir := filepath.Dir(exePath)
	if rel.ZipURL != "" && exePath != "" && canWriteDir(installDir) {
		zipPath, err := DownloadAndVerifyAsset(ctx, rel.ZipURL, rel.SumsURL, rel.ZipName, stageDir)
		if err != nil {
			return "", err
		}
		unpackedDir := filepath.Join(stageDir, "unpacked")
		if err := extractZipSafely(zipPath, unpackedDir); err != nil {
			return "", fmt.Errorf("解压更新包失败：%w", err)
		}
		if _, err := os.Stat(filepath.Join(unpackedDir, "mygo-run.exe")); err != nil {
			return "", fmt.Errorf("更新包内缺少 mygo-run.exe")
		}
		if err := applyDirectorySwap(unpackedDir, installDir); err != nil {
			return "", err
		}
		_ = os.RemoveAll(stageDir)
		// Wait briefly for current PID to exit and release RequestSingleInstanceLock before relaunch.
		relaunchPS := fmt.Sprintf("for ($i=0;$i -lt 40;$i++){if(-not (Get-Process -Id %d -ErrorAction SilentlyContinue)){break};Start-Sleep -Milliseconds 150}; Start-Process -FilePath '%s' -WorkingDirectory '%s'",
			pid,
			strings.ReplaceAll(exePath, "'", "''"),
			strings.ReplaceAll(installDir, "'", "''"),
		)
		cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", relaunchPS)
		if err := cmd.Start(); err != nil {
			return "", fmt.Errorf("更新文件已就绪，请手动重启应用：%w", err)
		}
		return "zip", nil
	}
	if rel.SetupURL != "" {
		setupPath, err := DownloadAndVerifyAsset(ctx, rel.SetupURL, rel.SumsURL, rel.SetupName, stageDir)
		if err != nil {
			return "", err
		}
		var args []string
		if exePath != "" {
			// Pre-select the running install folder (/D must be last without quotes in NSIS CLI).
			args = append(args, "/D="+installDir)
		}
		cmd := exec.Command(setupPath, args...)
		if err := cmd.Start(); err != nil {
			return "", fmt.Errorf("启动安装程序失败：%w", err)
		}
		return "installer", nil
	}
	return "", fmt.Errorf("当前版本未附带 Windows 安装包或便携压缩包")
}

// OpenURL opens an https release URL in the system default browser.
func OpenURL(rawURL string) error {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return fmt.Errorf("无效网页地址：%s", rawURL)
	}
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", u.String()).Start()
	case "darwin":
		return exec.Command("open", u.String()).Start()
	default:
		return exec.Command("xdg-open", u.String()).Start()
	}
}
