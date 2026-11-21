package diag

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	maxLogBytes   = 5 << 20 // 5 MB per log file
	maxLogBackups = 4
	maxRingLines  = 200
)

// CrashRecord persists the most recent unhandled panic for post-restart recovery banner.
type CrashRecord struct {
	Timestamp string `json:"timestamp"`
	Tag       string `json:"tag"`
	Summary   string `json:"summary"`
	CrashFile string `json:"crashFile"`
}

// EnvironmentMeta describes redacted runtime metadata packed into diagnostics/feedback.
type EnvironmentMeta struct {
	AppVersion  string `json:"appVersion"`
	BuildCommit string `json:"buildCommit"`
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	GoVersion   string `json:"goVersion"`
	VaultStats  string `json:"vaultStats"`
	DataDir     string `json:"dataDir,omitempty"`
}

var (
	mu          sync.Mutex
	activeDir   string
	activeFile  *os.File
	activeBytes int64
	ringBuf     []string

	winUserRe  = regexp.MustCompile(`(?i)[A-Z]:\\Users\\[^\\\s"']+`)
	macUserRe  = regexp.MustCompile(`/Users/[^/\s"']+`)
	nixUserRe  = regexp.MustCompile(`/home/[^/\s"']+`)
	ghTokenRe  = regexp.MustCompile(`(?:ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9_]{12,}|github_pat_[A-Za-z0-9_]{16,}`)
	bearerRe   = regexp.MustCompile(`(?i)(authorization:\s*bearer\s+|bearer\s+)[A-Za-z0-9._\-]{10,}`)
)

type teeWriter struct{}

func (t *teeWriter) Write(p []byte) (int, error) {
	text := string(p)
	sanitized := SanitizeText(text)
	mu.Lock()
	defer mu.Unlock()

	for _, line := range strings.Split(strings.TrimRight(sanitized, "\r\n"), "\n") {
		if line == "" {
			continue
		}
		ringBuf = append(ringBuf, line)
		if len(ringBuf) > maxRingLines {
			ringBuf = append([]string(nil), ringBuf[len(ringBuf)-maxRingLines:]...)
		}
	}

	if activeDir != "" && activeFile != nil {
		b := []byte(sanitized)
		if activeBytes+int64(len(b)) > maxLogBytes {
			rotateLocked()
		}
		if activeFile != nil {
			n, _ := activeFile.Write(b)
			activeBytes += int64(n)
		}
	}
	return len(p), nil
}

func rotateLocked() {
	if activeFile != nil {
		_ = activeFile.Close()
		activeFile = nil
	}
	base := filepath.Join(activeDir, "asterlab.log")
	for i := maxLogBackups; i >= 1; i-- {
		src := fmt.Sprintf("%s.%d", base, i)
		if i == maxLogBackups {
			_ = os.Remove(src)
			continue
		}
		dst := fmt.Sprintf("%s.%d", base, i+1)
		if _, err := os.Stat(src); err == nil {
			_ = os.Rename(src, dst)
		}
	}
	if _, err := os.Stat(base); err == nil {
		_ = os.Rename(base, base+".1")
	}
	f, err := os.OpenFile(base, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err == nil {
		activeFile = f
		activeBytes = 0
	}
}

// Init initializes rotating sanitized logging under logDir and attaches to standard log output.
func Init(logDir string) error {
	if strings.TrimSpace(logDir) == "" {
		return nil
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	if activeFile != nil {
		_ = activeFile.Close()
		activeFile = nil
	}
	activeDir = logDir
	base := filepath.Join(logDir, "asterlab.log")
	if st, err := os.Stat(base); err == nil {
		activeBytes = st.Size()
		if activeBytes >= maxLogBytes {
			rotateLocked()
		}
	}
	if activeFile == nil {
		f, err := os.OpenFile(base, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return err
		}
		activeFile = f
		if st, err := f.Stat(); err == nil {
			activeBytes = st.Size()
		}
	}
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetOutput(io.MultiWriter(os.Stderr, &teeWriter{}))
	return nil
}

// Close flushes and closes the active log file handle cleanly (helpful in tests).
func Close() {
	mu.Lock()
	defer mu.Unlock()
	if activeFile != nil {
		_ = activeFile.Close()
		activeFile = nil
	}
	activeDir = ""
	log.SetOutput(os.Stderr)
}

// LogDir returns the configured log directory.
func LogDir() string {
	mu.Lock()
	defer mu.Unlock()
	return activeDir
}

// RecentLines returns up to limit recent sanitized log lines from memory (or log file tail).
func RecentLines(limit int) []string {
	mu.Lock()
	dir := activeDir
	cp := append([]string(nil), ringBuf...)
	mu.Unlock()
	if len(cp) == 0 && dir != "" {
		if raw, err := os.ReadFile(filepath.Join(dir, "asterlab.log")); err == nil {
			for _, ln := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
				if ln != "" {
					cp = append(cp, SanitizeText(ln))
				}
			}
		}
	}
	if limit > 0 && len(cp) > limit {
		cp = cp[len(cp)-limit:]
	}
	return cp
}

// SanitizeText scrubs local home paths, OS usernames, and bearer/GitHub tokens.
func SanitizeText(s string) string {
	if s == "" {
		return ""
	}
	out := s
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		out = strings.ReplaceAll(out, home, "<USER_HOME>")
	}
	for _, envKey := range []string{"USERPROFILE", "LOCALAPPDATA", "APPDATA", "HOME"} {
		if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
			out = strings.ReplaceAll(out, v, "<"+envKey+">")
		}
	}
	out = winUserRe.ReplaceAllString(out, `<USER_HOME>`)
	out = macUserRe.ReplaceAllString(out, `/Users/<USER>`)
	out = nixUserRe.ReplaceAllString(out, `/home/<USER>`)
	if u, err := user.Current(); err == nil && u.Username != "" {
		short := u.Username
		if idx := strings.LastIndex(short, `\`); idx >= 0 {
			short = short[idx+1:]
		}
		if len(short) >= 3 {
			out = strings.ReplaceAll(out, short, "<USER>")
		}
	}
	out = ghTokenRe.ReplaceAllString(out, "<REDACTED_TOKEN>")
	out = bearerRe.ReplaceAllString(out, "${1}<REDACTED_TOKEN>")
	return out
}

// RecordPanic persists a sanitized crash report file plus last-crash.json marker.
func RecordPanic(tag string, recovered any, stack []byte) string {
	summary := SanitizeText(fmt.Sprintf("%v", recovered))
	body := fmt.Sprintf("Timestamp: %s\nTag: %s\nOS: %s/%s\nPanic: %s\n\nStack:\n%s\n",
		time.Now().UTC().Format(time.RFC3339),
		tag,
		runtime.GOOS, runtime.GOARCH,
		summary,
		SanitizeText(string(stack)),
	)
	mu.Lock()
	dir := activeDir
	mu.Unlock()
	if dir == "" {
		return ""
	}
	_ = os.MkdirAll(dir, 0o755)
	name := fmt.Sprintf("crash-%s.log", time.Now().UTC().Format("20060102-150405"))
	crashPath := filepath.Join(dir, name)
	_ = os.WriteFile(crashPath, []byte(body), 0o644)
	rec := CrashRecord{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Tag:       tag,
		Summary:   summary,
		CrashFile: name,
	}
	if raw, err := json.MarshalIndent(rec, "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(dir, "last-crash.json"), raw, 0o644)
	}
	pruneOldCrashLogs(dir, 10)
	return crashPath
}

func pruneOldCrashLogs(dir string, keep int) {
	matches, _ := filepath.Glob(filepath.Join(dir, "crash-*.log"))
	if len(matches) <= keep {
		return
	}
	sort.Strings(matches)
	for i := 0; i < len(matches)-keep; i++ {
		_ = os.Remove(matches[i])
	}
}

// RecoverAndLog is intended for `defer diag.RecoverAndLog("tag")` inside worker goroutines.
func RecoverAndLog(tag string) {
	if r := recover(); r != nil {
		buf := make([]byte, 16384)
		n := runtime.Stack(buf, false)
		path := RecordPanic(tag, r, buf[:n])
		log.Printf("recovered panic [%s]: %v (crash=%s)", tag, SanitizeText(fmt.Sprint(r)), filepath.Base(path))
	}
}

// PendingCrash returns an unread crash record from a previous run if present.
func PendingCrash(logDir string) *CrashRecord {
	if logDir == "" {
		logDir = LogDir()
	}
	if logDir == "" {
		return nil
	}
	raw, err := os.ReadFile(filepath.Join(logDir, "last-crash.json"))
	if err != nil {
		return nil
	}
	var rec CrashRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		return nil
	}
	return &rec
}

// DismissPendingCrash clears the unread crash notification marker while keeping the crash log file.
func DismissPendingCrash(logDir string) {
	if logDir == "" {
		logDir = LogDir()
	}
	if logDir == "" {
		return
	}
	_ = os.Remove(filepath.Join(logDir, "last-crash.json"))
}

// ExportBundle writes a privacy-scrubbed diagnostic ZIP containing logs, crash reports,
// sanitized update/app metadata, and NO .asterml experiment contents or tokens.
func ExportBundle(destZipPath string, meta EnvironmentMeta) error {
	if strings.TrimSpace(destZipPath) == "" {
		return fmt.Errorf("未指定诊断包输出路径")
	}
	if err := os.MkdirAll(filepath.Dir(destZipPath), 0o755); err != nil {
		return err
	}
	f, err := os.Create(destZipPath)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	addBytes := func(name string, data []byte) error {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}

	cleanMeta := meta
	cleanMeta.DataDir = ""
	cleanMeta.OS = runtime.GOOS
	cleanMeta.Arch = runtime.GOARCH
	cleanMeta.GoVersion = runtime.Version()
	if rawMeta, err := json.MarshalIndent(cleanMeta, "", "  "); err == nil {
		_ = addBytes("environment.json", []byte(SanitizeText(string(rawMeta))))
	}

	if lines := RecentLines(200); len(lines) > 0 {
		_ = addBytes("recent-memory.log", []byte(strings.Join(lines, "\n")+"\n"))
	}

	if dir := LogDir(); dir != "" {
		if raw, err := os.ReadFile(filepath.Join(dir, "asterlab.log")); err == nil {
			_ = addBytes("asterlab.log", []byte(SanitizeText(string(raw))))
		}
		crashes, _ := filepath.Glob(filepath.Join(dir, "crash-*.log"))
		sort.Strings(crashes)
		if len(crashes) > 5 {
			crashes = crashes[len(crashes)-5:]
		}
		for _, cp := range crashes {
			if raw, err := os.ReadFile(cp); err == nil {
				_ = addBytes("crashes/"+filepath.Base(cp), []byte(SanitizeText(string(raw))))
			}
		}
	}

	if meta.DataDir != "" {
		for _, cfg := range []string{"update-prefs.json", "update-state.json", "onboarding.json"} {
			p := filepath.Join(meta.DataDir, cfg)
			if raw, err := os.ReadFile(p); err == nil {
				_ = addBytes("config/"+cfg, []byte(SanitizeText(string(raw))))
			}
		}
	}

	if err := zw.Close(); err != nil {
		return err
	}
	return f.Close()
}

// BuildReportMarkdown renders a redacted markdown report suitable for clipboard or GitHub Issues.
func BuildReportMarkdown(category, description string, meta EnvironmentMeta, includeLogs bool) string {
	var b strings.Builder
	b.WriteString("### 问题描述\n")
	if strings.TrimSpace(description) == "" {
		b.WriteString("（请在此补充复现步骤与预期表现）\n\n")
	} else {
		b.WriteString(SanitizeText(strings.TrimSpace(description)) + "\n\n")
	}
	b.WriteString("### 运行环境（已自动脱敏）\n")
	b.WriteString(fmt.Sprintf("- 反馈类型：%s\n", category))
	b.WriteString(fmt.Sprintf("- 应用版本：%s (%s)\n", meta.AppVersion, meta.BuildCommit))
	b.WriteString(fmt.Sprintf("- 操作系统：%s/%s (%s)\n", runtime.GOOS, runtime.GOARCH, runtime.Version()))
	if meta.VaultStats != "" {
		b.WriteString(fmt.Sprintf("- 实验库规模：%s\n", SanitizeText(meta.VaultStats)))
	}
	if crash := PendingCrash(""); crash != nil {
		b.WriteString(fmt.Sprintf("- 最近异常：%s [%s] %s\n", crash.Timestamp, crash.Tag, crash.Summary))
	}
	if includeLogs {
		if lines := RecentLines(25); len(lines) > 0 {
			b.WriteString("\n### 最近日志摘要（最近 25 行）\n```text\n")
			b.WriteString(strings.Join(lines, "\n"))
			b.WriteString("\n```\n")
		}
	}
	return b.String()
}

// BuildGitHubIssueURL formats a pre-filled GitHub New Issue URL while respecting URL length limits.
func BuildGitHubIssueURL(repo, category, title, description string, meta EnvironmentMeta, includeLogs bool) string {
	if strings.TrimSpace(repo) == "" {
		repo = "LLLin000/AsterLab"
	}
	if strings.TrimSpace(title) == "" {
		title = fmt.Sprintf("[%s] %s 反馈", category, meta.AppVersion)
	}
	body := BuildReportMarkdown(category, description, meta, includeLogs)
	if len(body) > 3200 {
		body = body[:3200] + "\n...\n（日志已截断，请附加导出的诊断包 .zip）"
	}
	q := url.Values{}
	q.Set("title", title)
	q.Set("body", body)
	return fmt.Sprintf("https://github.com/%s/issues/new?%s", repo, q.Encode())
}
