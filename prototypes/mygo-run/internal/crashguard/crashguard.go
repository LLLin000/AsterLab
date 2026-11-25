// Package crashguard provides panic recovery with secret-sanitized local crash
// logging (`crash.log`) and atomic emergency draft staging for unsaved editor content.
package crashguard

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"asterlab-mygo-prototype/internal/asterdoc"
)

const (
	CrashLogFileName     = "crash.log"
	EmergencyDraftDirRel = ".asterlab/cache/emergency-drafts"
	maxCrashLogBytes     = 1 << 20 // 1 MiB rotation threshold
)

// EmergencyDraft stores unsaved editor content staged atomically to survive crashes.
type EmergencyDraft struct {
	DocRelPath   string    `json:"docRelPath"`
	Content      string    `json:"content"`
	CursorOffset int       `json:"cursorOffset,omitempty"`
	SavedAt      time.Time `json:"savedAt"`
	AppVersion   string    `json:"appVersion,omitempty"`
	SHA256       string    `json:"sha256"`
}

var (
	bearerTokenRe   = regexp.MustCompile(`(?i)\b(Bearer|Basic)\s+[A-Za-z0-9._~+/=-]{6,}`)
	urlCredsRe      = regexp.MustCompile(`(https?://)([^/\s:@]+):([^/\s@]+)@`)
	secretKeyValRe  = regexp.MustCompile(`(?i)\b(password|passwd|pwd|secret|token|api[_-]?key|auth[_-]?token|access[_-]?token|kandev[_-]?api[_-]?token|beta[_-]?feedback[_-]?key)\b(\s*[:=]\s*["']?)([^\s"',;}\]]+)`)
)

// RecoverAndWriteCrashLog is meant to be called directly with `defer`:
//
//	defer crashguard.RecoverAndWriteCrashLog(logDir, meta)
//
// When a panic occurs, it recovers it, sanitizes secrets from the panic text and
// stack trace, writes an entry to `<logDir>/crash.log`, and returns (true, path).
func RecoverAndWriteCrashLog(logDir string, meta ...map[string]string) (bool, string) {
	r := recover()
	if r == nil {
		return false, ""
	}
	var merged map[string]string
	if len(meta) > 0 {
		merged = meta[0]
	}
	path, _ := WriteCrashReport(logDir, r, debug.Stack(), merged)
	return true, path
}

// WriteCrashReport formats and appends a secret-sanitized panic report to `<logDir>/crash.log`.
func WriteCrashReport(logDir string, panicVal any, stack []byte, meta map[string]string) (string, error) {
	if strings.TrimSpace(logDir) == "" {
		return "", errors.New("crashguard: logDir is required")
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return "", err
	}
	logPath := filepath.Join(logDir, CrashLogFileName)
	rotateCrashLogIfLarge(logPath)

	var b strings.Builder
	fmt.Fprintf(&b, "=== AsterLab Crash Report @ %s ===\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "Platform: %s/%s (%s)\n", runtime.GOOS, runtime.GOARCH, runtime.Version())
	if len(meta) > 0 {
		keys := make([]string, 0, len(meta))
		for k := range meta {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			val := meta[k]
			if isSensitiveMetaKey(k) {
				val = "<redacted>"
			}
			line := SanitizeCrashText(fmt.Sprintf("%s: %s", k, val))
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	fmt.Fprintf(&b, "Panic: %s\n", SanitizeCrashText(fmt.Sprint(panicVal)))
	if len(stack) > 0 {
		fmt.Fprintf(&b, "Stack:\n%s\n", SanitizeCrashText(string(stack)))
	}
	b.WriteString("\n")

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.WriteString(b.String()); err != nil {
		return "", err
	}
	return logPath, nil
}

// SanitizeCrashText redacts credentials, bearer/basic tokens, URL passwords, and
// secret key/value pairs from crash logs so logs never leak sensitive tokens.
func SanitizeCrashText(raw string) string {
	s := urlCredsRe.ReplaceAllString(raw, `${1}<redacted>:<redacted>@`)
	s = bearerTokenRe.ReplaceAllString(s, `${1} <redacted>`)
	s = secretKeyValRe.ReplaceAllString(s, `${1}${2}<redacted>`)
	return s
}

// SaveEmergencyDraft atomically stages unsaved document content under
// `<baseDir>/.asterlab/cache/emergency-drafts/` with a SHA-256 integrity digest.
func SaveEmergencyDraft(baseDir, docRelPath, content string, cursorOffset int, appVersion string) (EmergencyDraft, error) {
	if strings.TrimSpace(baseDir) == "" || strings.TrimSpace(docRelPath) == "" {
		return EmergencyDraft{}, errors.New("crashguard: baseDir and docRelPath are required")
	}
	normRel := filepath.ToSlash(filepath.Clean(strings.TrimSpace(docRelPath)))
	sumBytes := sha256.Sum256([]byte(content))
	draft := EmergencyDraft{
		DocRelPath:   normRel,
		Content:      content,
		CursorOffset: cursorOffset,
		SavedAt:      time.Now().UTC(),
		AppVersion:   strings.TrimSpace(appVersion),
		SHA256:       hex.EncodeToString(sumBytes[:]),
	}
	raw, err := json.MarshalIndent(draft, "", "  ")
	if err != nil {
		return EmergencyDraft{}, err
	}
	if err := asterdoc.WriteFileAtomic(draftFilePath(baseDir, normRel), raw); err != nil {
		return EmergencyDraft{}, err
	}
	return draft, nil
}

// LoadEmergencyDraft reads and SHA-256-verifies a staged emergency draft for docRelPath.
func LoadEmergencyDraft(baseDir, docRelPath string) (EmergencyDraft, bool, error) {
	if strings.TrimSpace(baseDir) == "" || strings.TrimSpace(docRelPath) == "" {
		return EmergencyDraft{}, false, errors.New("crashguard: baseDir and docRelPath are required")
	}
	normRel := filepath.ToSlash(filepath.Clean(strings.TrimSpace(docRelPath)))
	raw, err := os.ReadFile(draftFilePath(baseDir, normRel))
	if errors.Is(err, os.ErrNotExist) {
		return EmergencyDraft{}, false, nil
	}
	if err != nil {
		return EmergencyDraft{}, false, err
	}
	var draft EmergencyDraft
	if err := json.Unmarshal(raw, &draft); err != nil {
		return EmergencyDraft{}, false, fmt.Errorf("crashguard: corrupt emergency draft: %w", err)
	}
	sumBytes := sha256.Sum256([]byte(draft.Content))
	got := hex.EncodeToString(sumBytes[:])
	if !strings.EqualFold(got, draft.SHA256) {
		return EmergencyDraft{}, false, fmt.Errorf("crashguard: emergency draft integrity mismatch (got %s want %s)", got, draft.SHA256)
	}
	return draft, true, nil
}

// ListEmergencyDrafts returns all valid staged emergency drafts under baseDir, newest first.
func ListEmergencyDrafts(baseDir string) ([]EmergencyDraft, error) {
	dir := filepath.Join(baseDir, filepath.FromSlash(EmergencyDraftDirRel))
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []EmergencyDraft
	for _, de := range entries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".json") {
			continue
		}
		raw, rErr := os.ReadFile(filepath.Join(dir, de.Name()))
		if rErr != nil {
			continue
		}
		var d EmergencyDraft
		if json.Unmarshal(raw, &d) != nil || d.DocRelPath == "" {
			continue
		}
		sum := sha256.Sum256([]byte(d.Content))
		if strings.EqualFold(hex.EncodeToString(sum[:]), d.SHA256) {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].SavedAt.After(out[j].SavedAt)
	})
	return out, nil
}

// ClearEmergencyDraft removes any staged emergency draft for docRelPath.
func ClearEmergencyDraft(baseDir, docRelPath string) error {
	if strings.TrimSpace(baseDir) == "" || strings.TrimSpace(docRelPath) == "" {
		return nil
	}
	normRel := filepath.ToSlash(filepath.Clean(strings.TrimSpace(docRelPath)))
	err := os.Remove(draftFilePath(baseDir, normRel))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func draftFilePath(baseDir, normRel string) string {
	h := sha256.Sum256([]byte(normRel))
	return filepath.Join(baseDir, filepath.FromSlash(EmergencyDraftDirRel), hex.EncodeToString(h[:12])+".json")
}

func rotateCrashLogIfLarge(logPath string) {
	info, err := os.Stat(logPath)
	if err != nil || info.Size() < maxCrashLogBytes {
		return
	}
	_ = os.Remove(logPath + ".1")
	_ = os.Rename(logPath, logPath+".1")
}

func isSensitiveMetaKey(key string) bool {
	lk := strings.ToLower(strings.TrimSpace(key))
	for _, needle := range []string{"password", "passwd", "secret", "token", "api_key", "apikey", "feedback_key", "auth"} {
		if strings.Contains(lk, needle) {
			return true
		}
	}
	return false
}
