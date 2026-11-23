// Command asterlab-feedback-relay is a minimal write-only HTTPS intake relay
// for AsterLab BetaFeedback builds. It verifies a write-only X-Feedback-Key,
// unpacks uploaded feedback .zip bundles safely into a private GitHub feedback
// repository (e.g. LLLin000/asterlab-feedback), pushes to main, and sends a
// Telegram notification with the submission ID so AI/developers can pull and
// fix issues immediately.
package main

import (
	"archive/zip"
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	maxUploadBytes     = 25 << 20 // 25 MiB compressed zip
	maxExtractedBytes  = 60 << 20 // 60 MiB uncompressed safety cap
	maxExtractedFiles  = 240
	rateLimitPerHourIP = 30
)

var (
	allowedExt = map[string]bool{
		".md":   true,
		".json": true,
		".png":  true,
		".txt":  true,
		".log":  true,
	}
	unsafeSlugRe = regexp.MustCompile(`[^\p{L}\p{N}._-]+`)
)

type relayConfig struct {
	ListenAddr   string
	UploadKey    string
	RepoDir      string
	RepoSlug     string
	GHUserName   string
	GHUserEmail  string
	GHToken      string
	TelegramBot  string
	TelegramChat string
}

type ipWindow struct {
	mu      sync.Mutex
	buckets map[string][]time.Time
}

func newIPWindow() *ipWindow {
	return &ipWindow{buckets: make(map[string][]time.Time)}
}

func (w *ipWindow) Allow(ip string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-time.Hour)
	prev := w.buckets[ip]
	kept := prev[:0]
	for _, ts := range prev {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	if len(kept) >= rateLimitPerHourIP {
		w.buckets[ip] = kept
		return false
	}
	w.buckets[ip] = append(kept, now)
	return true
}

type relayServer struct {
	cfg     relayConfig
	gitMu   sync.Mutex
	limiter *ipWindow
}

func main() {
	cfg := relayConfig{
		ListenAddr:   envOr("FEEDBACK_LISTEN_ADDR", "127.0.0.1:38435"),
		UploadKey:    strings.TrimSpace(os.Getenv("FEEDBACK_UPLOAD_KEY")),
		RepoDir:      envOr("FEEDBACK_REPO_DIR", "/srv/asterlab-feedback-repo"),
		RepoSlug:     envOr("FEEDBACK_REPO_SLUG", "LLLin000/asterlab-feedback"),
		GHUserName:   envOr("FEEDBACK_GIT_NAME", "AsterLab Feedback Bot"),
		GHUserEmail:  envOr("FEEDBACK_GIT_EMAIL", "feedback-bot@asterlab.party"),
		GHToken:      strings.TrimSpace(os.Getenv("GH_TOKEN")),
		TelegramBot:  strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		TelegramChat: strings.TrimSpace(os.Getenv("TELEGRAM_CHAT_ID")),
	}
	if cfg.UploadKey == "" {
		log.Fatal("FEEDBACK_UPLOAD_KEY is required")
	}

	srv := &relayServer{cfg: cfg, limiter: newIPWindow()}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/feedback/health", srv.handleHealth)
	mux.HandleFunc("/api/feedback", srv.handleUpload)

	httpSrv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 15 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      90 * time.Second,
	}
	log.Printf("asterlab-feedback-relay listening on %s -> %s", cfg.ListenAddr, cfg.RepoSlug)
	log.Fatal(httpSrv.ListenAndServe())
}

func (s *relayServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "service": "asterlab-feedback-relay"})
}

func (s *relayServer) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": "POST required"})
		return
	}
	key := strings.TrimSpace(r.Header.Get("X-Feedback-Key"))
	if len(key) != len(s.cfg.UploadKey) || subtle.ConstantTimeCompare([]byte(key), []byte(s.cfg.UploadKey)) != 1 {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "invalid feedback key"})
		return
	}
	ip := clientIP(r)
	if !s.limiter.Allow(ip) {
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"ok": false, "error": "rate limit exceeded"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes+512*1024)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": fmt.Sprintf("parse multipart: %v", err)})
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	file, _, err := r.FormFile("bundle")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "missing bundle file"})
		return
	}
	defer file.Close()
	zipBytes, err := io.ReadAll(io.LimitReader(file, maxUploadBytes+1))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "read bundle failed"})
		return
	}
	if len(zipBytes) == 0 || len(zipBytes) > maxUploadBytes {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"ok": false, "error": "bundle empty or exceeds 25MB limit"})
		return
	}

	testerRaw := strings.TrimSpace(r.FormValue("tester"))
	if testerRaw == "" {
		testerRaw = "anon"
	}
	versionRaw := strings.TrimSpace(r.FormValue("version"))
	if versionRaw == "" {
		versionRaw = "dev"
	}
	issueCount, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("issue_count")))
	hasCrash := strings.EqualFold(strings.TrimSpace(r.FormValue("has_crash")), "true")
	summaryRaw := strings.TrimSpace(r.FormValue("summary"))

	now := time.Now().UTC().In(time.FixedZone("CST", 8*3600))
	subID := "FB-" + now.Format("20060102-150405")
	dirName := fmt.Sprintf("%s-%s-%s", subID, safeSlug(testerRaw, 20), safeSlug(versionRaw, 24))
	relDir := filepath.ToSlash(filepath.Join("submissions", dirName))

	fileCount, err := s.commitZipToRepo(zipBytes, relDir, subID, testerRaw, versionRaw, issueCount, hasCrash)
	if err != nil {
		log.Printf("commit error [%s]: %v", subID, err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": fmt.Sprintf("store feedback failed: %v", err)})
		return
	}

	go s.notifyTelegram(subID, testerRaw, versionRaw, issueCount, fileCount, hasCrash, relDir, summaryRaw)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"id":         subID,
		"repo_path":  relDir,
		"file_count": fileCount,
	})
}

func (s *relayServer) commitZipToRepo(zipBytes []byte, relDir, subID, tester, version string, issueCount int, hasCrash bool) (int, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return 0, fmt.Errorf("invalid zip: %w", err)
	}
	if len(zr.File) > maxExtractedFiles {
		return 0, fmt.Errorf("too many files in zip (%d > %d)", len(zr.File), maxExtractedFiles)
	}

	s.gitMu.Lock()
	defer s.gitMu.Unlock()

	if err := s.ensureRepoSynced(); err != nil {
		return 0, err
	}

	destRoot := filepath.Join(s.cfg.RepoDir, filepath.FromSlash(relDir))
	if err := os.MkdirAll(destRoot, 0o755); err != nil {
		return 0, fmt.Errorf("mkdir submission dir: %w", err)
	}

	var totalBytes int64
	written := 0
	for _, zf := range zr.File {
		if zf.FileInfo().IsDir() {
			continue
		}
		if zf.Mode()&os.ModeSymlink != 0 {
			continue
		}
		clean := filepath.Clean(strings.ReplaceAll(zf.Name, "\\", "/"))
		if clean == "." || strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			return 0, fmt.Errorf("unsafe zip entry path: %s", zf.Name)
		}
		// Strip top-level wrapper folder if every file sits inside "asterlab-ui-feedback-.../"
		parts := strings.Split(filepath.ToSlash(clean), "/")
		if len(parts) > 1 && strings.HasPrefix(parts[0], "asterlab-ui-feedback-") {
			clean = filepath.Join(parts[1:]...)
		}
		ext := strings.ToLower(filepath.Ext(clean))
		if !allowedExt[ext] {
			continue
		}
		outPath := filepath.Join(destRoot, clean)
		if !strings.HasPrefix(outPath, destRoot+string(os.PathSeparator)) {
			return 0, fmt.Errorf("zip path escape: %s", zf.Name)
		}
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return 0, err
		}
		rc, err := zf.Open()
		if err != nil {
			return 0, err
		}
		data, readErr := io.ReadAll(io.LimitReader(rc, maxExtractedBytes-totalBytes+1))
		_ = rc.Close()
		if readErr != nil {
			return 0, readErr
		}
		totalBytes += int64(len(data))
		if totalBytes > maxExtractedBytes {
			return 0, errors.New("uncompressed zip exceeds 60MB safety cap")
		}
		if err := os.WriteFile(outPath, data, 0o644); err != nil {
			return 0, err
		}
		written++
	}
	if written == 0 {
		return 0, errors.New("zip contained no allowed feedback files")
	}

	// Write lightweight submission metadata header
	meta := map[string]any{
		"id":          subID,
		"tester":      tester,
		"version":     version,
		"issue_count": issueCount,
		"has_crash":   hasCrash,
		"received_at": time.Now().UTC().Format(time.RFC3339),
		"file_count":  written,
	}
	if raw, err := json.MarshalIndent(meta, "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(destRoot, "submission.json"), raw, 0o644)
	}

	if err := s.runGit("add", relDir); err != nil {
		return written, err
	}
	crashTag := ""
	if hasCrash {
		crashTag = " +crash"
	}
	msg := fmt.Sprintf("feedback(%s): %s (%d issues%s, %s)", subID, tester, issueCount, crashTag, version)
	if err := s.runGit("commit", "-m", msg); err != nil {
		return written, err
	}
	if err := s.runGit("push", "origin", "HEAD:main"); err != nil {
		return written, err
	}
	return written, nil
}

func (s *relayServer) ensureRepoSynced() error {
	if _, err := os.Stat(filepath.Join(s.cfg.RepoDir, ".git")); err != nil {
		if err := os.MkdirAll(filepath.Dir(s.cfg.RepoDir), 0o755); err != nil {
			return err
		}
		cloneURL := s.authenticatedRepoURL()
		cmd := exec.Command("git", "clone", cloneURL, s.cfg.RepoDir)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git clone: %v (%s)", err, redactToken(string(out), s.cfg.GHToken))
		}
		_ = s.runGit("config", "user.name", s.cfg.GHUserName)
		_ = s.runGit("config", "user.email", s.cfg.GHUserEmail)
		return nil
	}
	_ = s.runGit("remote", "set-url", "origin", s.authenticatedRepoURL())
	_ = s.runGit("config", "user.name", s.cfg.GHUserName)
	_ = s.runGit("config", "user.email", s.cfg.GHUserEmail)
	if err := s.runGit("pull", "--rebase", "--autostash", "origin", "main"); err != nil {
		return fmt.Errorf("git pull: %w", err)
	}
	return nil
}

func (s *relayServer) authenticatedRepoURL() string {
	if s.cfg.GHToken != "" {
		return fmt.Sprintf("https://x-access-token:%s@github.com/%s.git", s.cfg.GHToken, s.cfg.RepoSlug)
	}
	return fmt.Sprintf("https://github.com/%s.git", s.cfg.RepoSlug)
}

func (s *relayServer) runGit(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = s.cfg.RepoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %v (%s)", strings.Join(args, " "), err, redactToken(string(out), s.cfg.GHToken))
	}
	return nil
}

func (s *relayServer) notifyTelegram(subID, tester, version string, issueCount, fileCount int, hasCrash bool, relDir, summary string) {
	if s.cfg.TelegramBot == "" || s.cfg.TelegramChat == "" {
		return
	}
	crashLine := ""
	if hasCrash {
		crashLine = "\n⚠️ 包含崩溃堆栈日志 (logs/crash.log)"
	}
	if len([]rune(summary)) > 360 {
		summary = string([]rune(summary)[:360]) + "…"
	}
	summaryBlock := ""
	if summary != "" {
		summaryBlock = "\n\n摘要：\n" + summary
	}
	text := fmt.Sprintf("📩 收到 AsterLab 内测反馈 [%s]\n测试者：%s | 版本：%s\n记录：%d 条（%d 个文件）%s\n仓库：%s\n目录：%s%s\n\n👉 对 AI 说：拉取 %s 修复",
		subID, tester, version, issueCount, fileCount, crashLine, s.cfg.RepoSlug, relDir, summaryBlock, subID)

	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.cfg.TelegramBot)
	form := url.Values{}
	form.Set("chat_id", s.cfg.TelegramChat)
	form.Set("text", text)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.PostForm(endpoint, form)
	if err != nil {
		log.Printf("telegram notify failed: %v", err)
		return
	}
	_ = resp.Body.Close()
}

func safeSlug(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	s = unsafeSlugRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-._")
	if s == "" {
		return "unknown"
	}
	rs := []rune(s)
	if len(rs) > maxRunes {
		rs = rs[:maxRunes]
	}
	return string(rs)
}

func clientIP(r *http.Request) string {
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		if idx := strings.IndexByte(xff, ','); idx > 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return xff
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func redactToken(s, tok string) string {
	if tok == "" {
		return s
	}
	return strings.ReplaceAll(s, tok, "<REDACTED>")
}

func envOr(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
