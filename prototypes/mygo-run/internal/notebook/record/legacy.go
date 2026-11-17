package record

import (
	"regexp"
	"strings"
)

// legacySignatureRe matches the old prose-signature trailers
// (`<!-- record-sign run=… rev=… hash=… at=… actor=… -->`) that notebook
// pages persisted before sign/archive moved to Core commands. The HTML
// comment is inert: it is never verified, never a lock, never authority —
// only persistent Core state (run_archive_state) is. This helper strips the
// trailer at display/export boundaries so old content stays readable while
// the comment itself remains history-only.
var legacySignatureRe = regexp.MustCompile(`(?s)<!--\s*record-sign\s+run=(\S+)\s+rev=(\d+)\s+hash=([0-9a-f]{64})\s+at=(\S+)\s+actor=(.+?)\s*-->[ \t]*$`)

// StripLegacySignature removes a trailing legacy signature comment, if any.
// Prose without one passes through untouched.
func StripLegacySignature(content string) string {
	return strings.TrimSpace(legacySignatureRe.ReplaceAllString(content, ""))
}
