package runworkspace

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// Bridge v2 generic-channel kinds consumed by the Run surface. The strings
// are Core's command/query names (INT-01, mygo-v1-contract §Bridge v2); Go
// never invents new ones and never re-derives the numbers Core returns.
const (
	channelQueryWorkspace   = "run.workspace"
	channelCommandExecEvent = "execution_event.create"
	channelCommandReadout   = "readout.submit"
)

// newIdempotencyKey mints one key per user action (never per retry): the same
// action retried reuses its key, a new action mints a new key. Core replays
// the first result for a repeated key and rejects a reused key with a
// different payload, so the key is the action's identity.
func newIdempotencyKey() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("ui-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

// channelError maps a Bridge v2 rejection to operator wording (UX-P5,
// UX-STATE): every failure names its cause and offers a retry, branching on
// Core's stable error code only, never on message text.
func channelError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "NOT_FOUND"):
		return "对象不存在，请刷新后重试"
	case strings.Contains(msg, "REVISION_CONFLICT") ||
		strings.Contains(msg, "VERSION_CONFLICT") ||
		strings.Contains(msg, "STALE"):
		return "数据已被更新，请刷新后重试"
	case strings.Contains(msg, "IDEMPOTENCY_MISMATCH"):
		return "该操作已提交过不同内容，请刷新后重试"
	case strings.Contains(msg, "INVALID"):
		return "数据未通过实验规则核查（" + msg + "），请核对后重试"
	default:
		return msg
	}
}
