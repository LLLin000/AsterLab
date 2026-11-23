package devfeedback

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Build-time injectable write-only BetaFeedback relay settings.
// Public release builds (-tags=mygo_noinspector) do not even compile this package's UI callers.
// Even inside BetaFeedback builds, UploadKey is strictly write-only (cannot read repo or invoke GitHub/Telegram).
var (
	DefaultUploadEndpoint = ""
	DefaultUploadKey      = ""
)

// UploadResponse is returned by the VPS feedback intake relay.
type UploadResponse struct {
	OK        bool   `json:"ok"`
	ID        string `json:"id"`
	RepoPath  string `json:"repo_path"`
	FileCount int    `json:"file_count"`
	Error     string `json:"error,omitempty"`
}

// UploadBundle posts a zipped feedback bundle to the configured write-only relay endpoint.
func UploadBundle(endpoint, uploadKey, zipPath, tester, version string, issueCount int, hasCrash bool, summary string) (UploadResponse, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return UploadResponse{}, fmt.Errorf("未配置反馈上报地址")
	}
	if strings.TrimSpace(uploadKey) == "" {
		return UploadResponse{}, fmt.Errorf("未配置上报口令")
	}
	zipBytes, err := os.ReadFile(zipPath)
	if err != nil {
		return UploadResponse{}, fmt.Errorf("读取反馈包失败: %w", err)
	}
	if len(zipBytes) == 0 {
		return UploadResponse{}, fmt.Errorf("反馈包为空")
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("tester", strings.TrimSpace(tester))
	_ = mw.WriteField("version", strings.TrimSpace(version))
	_ = mw.WriteField("issue_count", strconv.Itoa(issueCount))
	if hasCrash {
		_ = mw.WriteField("has_crash", "true")
	} else {
		_ = mw.WriteField("has_crash", "false")
	}
	if strings.TrimSpace(summary) != "" {
		_ = mw.WriteField("summary", strings.TrimSpace(summary))
	}
	part, err := mw.CreateFormFile("bundle", filepath.Base(zipPath))
	if err != nil {
		return UploadResponse{}, fmt.Errorf("构造上传表单失败: %w", err)
	}
	if _, err := part.Write(zipBytes); err != nil {
		return UploadResponse{}, fmt.Errorf("写入反馈包数据失败: %w", err)
	}
	if err := mw.Close(); err != nil {
		return UploadResponse{}, err
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, &body)
	if err != nil {
		return UploadResponse{}, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-Feedback-Key", strings.TrimSpace(uploadKey))

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return UploadResponse{}, fmt.Errorf("连接反馈服务器失败: %w", err)
	}
	defer resp.Body.Close()

	respRaw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var out UploadResponse
	_ = json.Unmarshal(respRaw, &out)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !out.OK {
		msg := out.Error
		if msg == "" {
			msg = strings.TrimSpace(string(respRaw))
		}
		if msg == "" {
			msg = resp.Status
		}
		return out, fmt.Errorf("服务器返回错误 (%d): %s", resp.StatusCode, msg)
	}
	return out, nil
}

// CopyFileToSystemClipboard puts zipPath onto the OS file clipboard (Windows Set-Clipboard -Path
// so testers can paste the .zip directly into WeChat / Feishu / Email with Ctrl+V).
func CopyFileToSystemClipboard(zipPath string) error {
	absPath, err := filepath.Abs(zipPath)
	if err != nil {
		absPath = zipPath
	}
	switch runtime.GOOS {
	case "windows":
		escaped := strings.ReplaceAll(absPath, "'", "''")
		cmd := exec.Command("powershell.exe", "-NoProfile", "-WindowStyle", "Hidden", "-Command",
			fmt.Sprintf("Set-Clipboard -LiteralPath '%s'", escaped))
		return cmd.Run()
	case "darwin":
		escaped := strings.ReplaceAll(absPath, `"`, `\"`)
		cmd := exec.Command("osascript", "-e", fmt.Sprintf(`set the clipboard to (POSIX file "%s")`, escaped))
		return cmd.Run()
	default:
		return nil
	}
}
