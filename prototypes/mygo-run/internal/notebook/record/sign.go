package record

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// RecordState is the shared status vocabulary for the record surfaces. Every
// surface names the state in the same words (docs/product/ux-contract.md §3):
// 未保存 (dirty prose), 已保存 (persisted, no pending change), 已签核归档
// (signed and locked: the prose verifies against its signature).
type RecordState string

const (
	// StateUnsaved means the prose changed since it was last persisted.
	StateUnsaved RecordState = "未保存"
	// StateSaved means the prose is persisted with no pending change.
	StateSaved RecordState = "已保存"
	// StateSealed means the prose carries a valid signature and is locked
	// against further edits until explicitly unlocked.
	StateSealed RecordState = "已签核归档"
)

// ClassifyRecordState derives the state word from the dirty flag and the
// signature check. A valid signature over unchanged prose outranks everything;
// anything else dirty is 未保存.
func ClassifyRecordState(dirty bool, signatureOK bool) RecordState {
	if signatureOK && !dirty {
		return StateSealed
	}
	if dirty {
		return StateUnsaved
	}
	return StateSaved
}

// Signature records who signed a Notebook prose revision and what it covered.
// The signature travels as a trailing HTML comment in the prose itself, so it
// persists through the existing prose-writing path (SaveNotebook) with no new
// Core command and no second store.
type Signature struct {
	RunID    string
	Revision uint64
	Hash     string
	At       string
	Actor    string
}

// actor is the last field and may contain internal spaces (Windows USERNAME
// can be "Jane Doe"), so it captures everything up to the closing comment.
// Signatures written before spaces were allowed (actor=\S+) still match.
var signatureRe = regexp.MustCompile(`(?s)<!--\s*record-sign\s+run=(\S+)\s+rev=(\d+)\s+hash=([0-9a-f]{64})\s+at=(\S+)\s+actor=(.+?)\s*-->[ \t]*$`)

// signatureBody strips any existing signature trailer so re-signing covers
// the prose, not the previous signature.
func signatureBody(content string) string {
	trimmed := strings.TrimRight(content, " \t\n")
	if loc := signatureRe.FindStringIndex(trimmed); loc != nil {
		return strings.TrimRight(trimmed[:loc[0]], " \t\n") + "\n"
	}
	if trimmed == "" {
		return ""
	}
	return trimmed + "\n"
}

// Sign appends a signature trailer covering the prose body. The returned
// string is what gets persisted; the prose stays human-readable Markdown with
// one machine comment at the end.
func Sign(content, runID string, revision uint64, actor string, at time.Time) (string, Signature) {
	body := signatureBody(content)
	sum := sha256.Sum256([]byte(body))
	sig := Signature{
		RunID:    runID,
		Revision: revision,
		Hash:     hex.EncodeToString(sum[:]),
		At:       at.UTC().Format(time.RFC3339),
		Actor:    actor,
	}
	trailer := fmt.Sprintf("<!-- record-sign run=%s rev=%d hash=%s at=%s actor=%s -->",
		sig.RunID, sig.Revision, sig.Hash, sig.At, sig.Actor)
	return body + "\n" + trailer + "\n", sig
}

// Verify checks the trailing signature against the prose body it covers.
// ok is false when there is no trailer or the prose changed since signing.
func Verify(content string) (Signature, bool) {
	trimmed := strings.TrimRight(content, " \t\n")
	match := signatureRe.FindStringSubmatch(trimmed)
	if match == nil {
		return Signature{}, false
	}
	var rev uint64
	if _, err := fmt.Sscanf(match[2], "%d", &rev); err != nil {
		return Signature{}, false
	}
	sig := Signature{RunID: match[1], Revision: rev, Hash: match[3], At: match[4], Actor: match[5]}
	body := strings.TrimRight(trimmed[:strings.LastIndex(trimmed, "<!--")], " \t\n") + "\n"
	if body == "\n" {
		body = ""
	}
	sum := sha256.Sum256([]byte(body))
	if hex.EncodeToString(sum[:]) != sig.Hash {
		return Signature{}, false
	}
	return sig, true
}

// StripSignature removes the signature trailer, returning the prose to the
// editable 未保存 state. Unlocking never edits the prose itself.
func StripSignature(content string) string {
	return signatureBody(content)
}
