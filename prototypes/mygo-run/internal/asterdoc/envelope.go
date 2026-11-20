package asterdoc

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// EnvelopeFormatVersion identifies the typed-block single-file AsterML envelope.
const EnvelopeFormatVersion = "2.1"

// EnvelopeSeal records tamper-evident cryptographic digests for the locked Design
// blueprint and Execution fact ledger inside a single `.asterml` file.
type EnvelopeSeal struct {
	DesignSHA256    string `yaml:"design_sha256,omitempty" json:"design_sha256,omitempty"`
	FrozenAt        string `yaml:"frozen_at,omitempty" json:"frozen_at,omitempty"`
	ExecutionSHA256 string `yaml:"execution_sha256,omitempty" json:"execution_sha256,omitempty"`
}

// EnvelopeFrontmatter is the YAML control plane at byte 0 of an `.asterml` file.
// Calendar views can index metadata in O(1) without parsing step blocks.
type EnvelopeFrontmatter struct {
	AsterML       string        `yaml:"asterml,omitempty" json:"asterml,omitempty"`
	ID            string        `yaml:"id,omitempty" json:"id,omitempty"`
	Title         string        `yaml:"title,omitempty" json:"title,omitempty"`
	ScheduledDate string        `yaml:"scheduled_date,omitempty" json:"scheduled_date,omitempty"`
	Stage         string        `yaml:"stage,omitempty" json:"stage,omitempty"`
	Tags          []string      `yaml:"tags,omitempty" json:"tags,omitempty"`
	RepeatOf      string        `yaml:"repeat_of,omitempty" json:"repeat_of,omitempty"`
	Seal          *EnvelopeSeal `yaml:"seal,omitempty" json:"seal,omitempty"`
}

// ExecutionLedger holds typed operational facts persisted in ```asterlab-exec```
// inside a single `.asterml` file without duplicating the Design blueprint.
type ExecutionLedger struct {
	Schema       int              `yaml:"schema" json:"schema"`
	RunID        string           `yaml:"run_id,omitempty" json:"run_id,omitempty"`
	DesignSHA256 string           `yaml:"design_sha256,omitempty" json:"design_sha256,omitempty"`
	Status       string           `yaml:"status,omitempty" json:"status,omitempty"`
	UpdatedAt    string           `yaml:"updated_at,omitempty" json:"updated_at,omitempty"`
	Checks       map[string]string `yaml:"checks,omitempty" json:"checks,omitempty"`
	ActualParams map[string]any   `yaml:"actual_params,omitempty" json:"actual_params,omitempty"`
	Timers       []map[string]any `yaml:"timers,omitempty" json:"timers,omitempty"`
	Readouts     []map[string]any `yaml:"readouts,omitempty" json:"readouts,omitempty"`
	Assets       []map[string]any `yaml:"assets,omitempty" json:"assets,omitempty"`
	Deviations   []map[string]any `yaml:"deviations,omitempty" json:"deviations,omitempty"`
}

// RecordSection holds human/ELN synthesis persisted in ````asterlab-record````
// (4 backticks so embedded ```asterlab``` widgets inside notes never collide).
type RecordSection struct {
	Schema     int    `yaml:"schema" json:"schema"`
	RunID      string `yaml:"run_id,omitempty" json:"run_id,omitempty"`
	UpdatedAt  string `yaml:"updated_at,omitempty" json:"updated_at,omitempty"`
	Markdown   string `yaml:"-" json:"markdown"`
	FenceStart int    `yaml:"-" json:"-"`
	FenceEnd   int    `yaml:"-" json:"-"`
}

// Envelope represents the parsed multi-stage structure of one single `.asterml` file.
type Envelope struct {
	Raw            string
	Frontmatter    EnvelopeFrontmatter
	HasFrontmatter bool
	FrontmatterEnd int
	DesignSource   string
	DesignEnd      int // byte offset in Raw where Design slice ends (before exec/record blocks)
	Execution      *ExecutionLedger
	ExecStart      int
	ExecEnd        int
	Record         *RecordSection
}

// ScanFrontmatter reads only the leading YAML frontmatter control plane (O(1) prefix scan).
func ScanFrontmatter(raw string) (EnvelopeFrontmatter, int, bool) {
	var fm EnvelopeFrontmatter
	work := raw
	bomLen := 0
	if strings.HasPrefix(work, "\ufeff") {
		bomLen = len("\ufeff")
		work = work[bomLen:]
	}
	lines := strings.SplitAfter(work, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return fm, 0, false
	}
	var yamlBuf strings.Builder
	offset := len(lines[0])
	closed := false
	for i := 1; i < len(lines); i++ {
		line := lines[i]
		tr := strings.TrimSpace(line)
		offset += len(line)
		if tr == "---" || tr == "..." {
			closed = true
			break
		}
		if strings.HasPrefix(tr, "```") {
			return EnvelopeFrontmatter{}, 0, false
		}
		yamlBuf.WriteString(line)
	}
	if !closed {
		return EnvelopeFrontmatter{}, 0, false
	}
	var m map[string]any
	if err := yaml.Unmarshal([]byte(yamlBuf.String()), &m); err != nil {
		return EnvelopeFrontmatter{}, 0, false
	}
	// Only treat as AsterML envelope frontmatter if it declares recognizable envelope keys
	// or is cleanly empty metadata (never hijack an unrelated horizontal-rule Markdown block).
	if len(m) == 0 {
		return EnvelopeFrontmatter{}, 0, false
	}
	hasKnown := false
	for _, k := range []string{"asterml", "id", "title", "date", "scheduled_date", "status", "stage", "repeat_of", "seal"} {
		if _, ok := m[k]; ok {
			hasKnown = true
			break
		}
	}
	if !hasKnown {
		return EnvelopeFrontmatter{}, 0, false
	}
	if err := yaml.Unmarshal([]byte(yamlBuf.String()), &fm); err != nil {
		return EnvelopeFrontmatter{}, 0, false
	}
	if rawDate, ok := m["scheduled_date"]; ok {
		fm.ScheduledDate = NormalizeDateYMD(formatYAMLDateValue(rawDate))
	} else if rawDate, ok := m["date"]; ok {
		fm.ScheduledDate = NormalizeDateYMD(formatYAMLDateValue(rawDate))
	}
	if fm.Stage == "" {
		if st, ok := m["status"].(string); ok {
			fm.Stage = strings.TrimSpace(st)
		}
	}
	return fm, bomLen + offset, true
}

func formatYAMLDateValue(v any) string {
	switch tv := v.(type) {
	case time.Time:
		return tv.Format("2006-01-02")
	case string:
		return tv
	default:
		return fmt.Sprint(v)
	}
}

// ComputeDesignSeal computes a canonical SHA-256 hex digest over all Layer-1/2 Design
// `asterlab` modules so seals remain stable across harmless whitespace formatting.
func ComputeDesignSeal(designSource string) string {
	doc, err := Parse(designSource)
	h := sha256.New()
	if err == nil && len(doc.Modules) > 0 {
		for _, m := range doc.Modules {
			h.Write([]byte(m.ID))
			h.Write([]byte{0})
			canon, mErr := yaml.Marshal(m.Data)
			if mErr == nil {
				h.Write(canon)
			} else {
				h.Write([]byte(strings.ReplaceAll(strings.TrimSpace(m.Body), "\r\n", "\n")))
			}
			h.Write([]byte{0})
		}
		return hex.EncodeToString(h.Sum(nil))
	}
	normalized := strings.TrimSpace(strings.ReplaceAll(designSource, "\r\n", "\n"))
	h.Write([]byte(normalized))
	return hex.EncodeToString(h.Sum(nil))
}

func computeExecutionDigest(exec ExecutionLedger) (string, error) {
	if exec.Schema <= 0 {
		exec.Schema = 1
	}
	raw, err := yaml.Marshal(exec)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// VerifyDesignSeal reports whether the envelope declares a Design seal and whether
// the current Design section matches the sealed SHA-256 digest.
func (e *Envelope) VerifyDesignSeal() (sealed bool, valid bool) {
	if e == nil || e.Frontmatter.Seal == nil || strings.TrimSpace(e.Frontmatter.Seal.DesignSHA256) == "" {
		return false, true
	}
	actual := ComputeDesignSeal(e.DesignSource)
	return true, actual == strings.TrimSpace(e.Frontmatter.Seal.DesignSHA256)
}

// ParseEnvelope splits a single `.asterml` file into Frontmatter, DesignSource,
// ExecutionLedger (`asterlab-exec`), and RecordSection (`asterlab-record`).
// Plain `.asterml` files without frontmatter/exec/record parse cleanly as unsealed Design files.
func ParseEnvelope(raw string) (*Envelope, error) {
	env := &Envelope{Raw: raw}
	fm, fmEnd, hasFM := ScanFrontmatter(raw)
	env.Frontmatter = fm
	env.HasFrontmatter = hasFM
	env.FrontmatterEnd = fmEnd

	rest := raw[fmEnd:]
	lines := strings.SplitAfter(rest, "\n")
	offset := fmEnd
	type span struct{ start, end int }
	var removeSpans []span
	firstTailStart := len(raw)

	inExec := false
	inRecord := false
	inOther := false
	fenceTicks := 0
	fenceStart := 0
	var body strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inExec && !inRecord && !inOther {
			if ticks := countLeadingBackticks(trimmed); ticks >= 3 {
				lang := strings.TrimSpace(trimmed[ticks:])
				switch lang {
				case "asterlab-exec":
					inExec = true
					fenceTicks = ticks
					fenceStart = offset
					body.Reset()
				case "asterlab-record":
					inRecord = true
					fenceTicks = ticks
					fenceStart = offset
					body.Reset()
				default:
					inOther = true
					fenceTicks = ticks
				}
			}
			offset += len(line)
			continue
		}
		if inOther {
			if isClosingBacktickFence(trimmed, fenceTicks) {
				inOther = false
				fenceTicks = 0
			}
			offset += len(line)
			continue
		}
		if inExec {
			if isClosingBacktickFence(trimmed, fenceTicks) {
				var ledger ExecutionLedger
				if err := yaml.Unmarshal([]byte(body.String()), &ledger); err != nil {
					return nil, fmt.Errorf("asterlab-exec block: %w", err)
				}
				if ledger.Schema == 0 {
					ledger.Schema = 1
				}
				fenceEnd := offset + len(line)
				env.Execution = &ledger
				env.ExecStart = fenceStart
				env.ExecEnd = fenceEnd
				removeSpans = append(removeSpans, span{fenceStart, fenceEnd})
				if fenceStart < firstTailStart {
					firstTailStart = fenceStart
				}
				inExec = false
				fenceTicks = 0
				body.Reset()
			} else {
				body.WriteString(line)
			}
			offset += len(line)
			continue
		}
		if inRecord {
			if isClosingBacktickFence(trimmed, fenceTicks) {
				rec, err := parseRecordBody(body.String())
				if err != nil {
					return nil, fmt.Errorf("asterlab-record block: %w", err)
				}
				fenceEnd := offset + len(line)
				rec.FenceStart = fenceStart
				rec.FenceEnd = fenceEnd
				env.Record = &rec
				removeSpans = append(removeSpans, span{fenceStart, fenceEnd})
				if fenceStart < firstTailStart {
					firstTailStart = fenceStart
				}
				inRecord = false
				fenceTicks = 0
				body.Reset()
			} else {
				body.WriteString(line)
			}
			offset += len(line)
		}
	}

	if inExec {
		return nil, errors.New("asterlab-exec block is not closed")
	}
	if inRecord {
		return nil, errors.New("asterlab-record block is not closed")
	}

	env.DesignEnd = firstTailStart
	if len(removeSpans) == 0 {
		env.DesignSource = strings.TrimLeft(rest, "\r\n")
	} else {
		var designOut strings.Builder
		cursor := fmEnd
		for _, sp := range removeSpans {
			if sp.start > cursor {
				designOut.WriteString(raw[cursor:sp.start])
			}
			if sp.end > cursor {
				cursor = sp.end
			}
		}
		if cursor < len(raw) {
			designOut.WriteString(raw[cursor:])
		}
		trimmedDesign := strings.TrimSpace(designOut.String())
		if trimmedDesign != "" {
			env.DesignSource = trimmedDesign + "\n"
		}
	}
	return env, nil
}

func parseRecordBody(body string) (RecordSection, error) {
	rec := RecordSection{Schema: 1}
	norm := strings.ReplaceAll(body, "\r\n", "\n")
	lines := strings.SplitAfter(norm, "\n")
	splitIdx := -1
	offset := 0
	for i, line := range lines {
		if strings.TrimSpace(line) == "---" {
			splitIdx = i
			break
		}
		offset += len(line)
	}
	if splitIdx >= 0 {
		hdrYAML := strings.Join(lines[:splitIdx], "")
		if strings.TrimSpace(hdrYAML) != "" {
			if err := yaml.Unmarshal([]byte(hdrYAML), &rec); err != nil {
				return rec, err
			}
		}
		if rec.Schema == 0 {
			rec.Schema = 1
		}
		mdStart := offset + len(lines[splitIdx])
		rec.Markdown = strings.TrimRight(norm[mdStart:], "\n")
		return rec, nil
	}
	rec.Markdown = strings.TrimRight(norm, "\n")
	return rec, nil
}

// ComposeEnvelope serializes Frontmatter, pure Design source, optional ExecutionLedger
// (```asterlab-exec```), and optional RecordSection (````asterlab-record````) into
// one self-contained canonical `.asterml` UTF-8 file.
func ComposeEnvelope(fm EnvelopeFrontmatter, designSource string, exec *ExecutionLedger, rec *RecordSection) (string, error) {
	if strings.TrimSpace(fm.AsterML) == "" {
		fm.AsterML = EnvelopeFormatVersion
	}
	if fm.ScheduledDate != "" {
		if norm := NormalizeDateYMD(fm.ScheduledDate); norm != "" {
			fm.ScheduledDate = norm
		}
	}
	fmBytes, err := yaml.Marshal(fm)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	out.WriteString("---\n")
	out.WriteString(strings.TrimRight(string(fmBytes), "\r\n"))
	out.WriteString("\n---\n")
	cleanDesign := strings.Trim(strings.ReplaceAll(designSource, "\r\n", "\n"), "\n")
	if cleanDesign != "" {
		out.WriteString(cleanDesign)
		out.WriteString("\n")
	}
	if exec != nil {
		eCopy := *exec
		if eCopy.Schema <= 0 {
			eCopy.Schema = 1
		}
		execBytes, err := yaml.Marshal(eCopy)
		if err != nil {
			return "", err
		}
		out.WriteString("\n```asterlab-exec\n")
		out.WriteString(strings.TrimRight(string(execBytes), "\r\n"))
		out.WriteString("\n```\n")
	}
	if rec != nil {
		rCopy := *rec
		if rCopy.Schema <= 0 {
			rCopy.Schema = 1
		}
		hdrBytes, err := yaml.Marshal(rCopy)
		if err != nil {
			return "", err
		}
		out.WriteString("\n````asterlab-record\n")
		out.WriteString(strings.TrimRight(string(hdrBytes), "\r\n"))
		out.WriteString("\n---\n")
		md := strings.TrimRight(strings.ReplaceAll(rCopy.Markdown, "\r\n", "\n"), "\n")
		if md != "" {
			out.WriteString(md)
			out.WriteString("\n")
		}
		out.WriteString("````\n")
	}
	return out.String(), nil
}

// PatchEnvelopeExecution surgically updates only Frontmatter + ```asterlab-exec```
// while preserving the raw Design span byte-for-byte (never invalidating DesignSHA256).
func PatchEnvelopeExecution(raw string, exec ExecutionLedger) (string, error) {
	env, err := ParseEnvelope(raw)
	if err != nil {
		return "", err
	}
	fm := env.Frontmatter
	if strings.TrimSpace(fm.AsterML) == "" {
		fm.AsterML = EnvelopeFormatVersion
	}
	if strings.TrimSpace(exec.DesignSHA256) == "" {
		if fm.Seal != nil && fm.Seal.DesignSHA256 != "" {
			exec.DesignSHA256 = fm.Seal.DesignSHA256
		} else {
			exec.DesignSHA256 = ComputeDesignSeal(env.DesignSource)
		}
	}
	if fm.Seal == nil {
		fm.Seal = &EnvelopeSeal{}
	}
	if strings.TrimSpace(fm.Seal.DesignSHA256) == "" {
		fm.Seal.DesignSHA256 = exec.DesignSHA256
	}
	if strings.TrimSpace(fm.Seal.FrozenAt) == "" {
		fm.Seal.FrozenAt = time.Now().UTC().Format(time.RFC3339)
	}
	execDigest, err := computeExecutionDigest(exec)
	if err == nil {
		fm.Seal.ExecutionSHA256 = execDigest
	}
	switch exec.Status {
	case "completed":
		fm.Stage = "completed"
	case "in_progress", "planned":
		if fm.Stage != "completed" {
			fm.Stage = "in_progress"
		}
	}
	designSlice := env.DesignSource
	if env.DesignEnd > env.FrontmatterEnd {
		designSlice = raw[env.FrontmatterEnd:env.DesignEnd]
	}
	return ComposeEnvelope(fm, designSlice, &exec, env.Record)
}

// PatchEnvelopeRecord surgically updates only Frontmatter + ````asterlab-record````
// while preserving both the Design span and any Execution ledger untouched.
func PatchEnvelopeRecord(raw string, rec RecordSection) (string, error) {
	env, err := ParseEnvelope(raw)
	if err != nil {
		return "", err
	}
	fm := env.Frontmatter
	if strings.TrimSpace(fm.AsterML) == "" {
		fm.AsterML = EnvelopeFormatVersion
	}
	designSlice := env.DesignSource
	if env.DesignEnd > env.FrontmatterEnd {
		designSlice = raw[env.FrontmatterEnd:env.DesignEnd]
	}
	return ComposeEnvelope(fm, designSlice, env.Execution, &rec)
}

// ForkEnvelopeAsDraft clones the Design blueprint of an existing experiment envelope into
// a brand-new unsealed draft `.asterml` file, recording `repeat_of` provenance while clearing
// execution history, seal, and notebook records.
func ForkEnvelopeAsDraft(raw string, newID, scheduledDate string) (string, error) {
	env, err := ParseEnvelope(raw)
	if err != nil {
		return "", err
	}
	designSrc := strings.TrimSpace(env.DesignSource)
	if dateNorm := NormalizeDateYMD(scheduledDate); dateNorm != "" {
		designSrc = StampScheduledDate(designSrc, dateNorm)
	}
	fm := EnvelopeFrontmatter{
		AsterML:       EnvelopeFormatVersion,
		ID:            strings.TrimSpace(newID),
		Title:         env.Frontmatter.Title,
		ScheduledDate: NormalizeDateYMD(scheduledDate),
		Stage:         "draft",
		RepeatOf:      strings.TrimSpace(env.Frontmatter.ID),
		Seal:          nil,
	}
	if fm.Title == "" {
		fm.Title = InspectExperiment(designSrc, nil).Title
	}
	return ComposeEnvelope(fm, designSrc, nil, nil)
}

// WriteFileAtomic writes data safely via a unique temp file + sync + rename in the same directory.
// Using os.CreateTemp prevents concurrent writers from corrupting a shared ".tmp" file,
// and short retries handle transient Windows antivirus/indexer file locks on os.Rename.
func WriteFileAtomic(targetPath string, data []byte) error {
	if strings.TrimSpace(targetPath) == "" {
		return errors.New("empty target path")
	}
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".asterlab-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	cleanup := func() { _ = os.Remove(tmp) }
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		cleanup()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		cleanup()
		return err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return err
	}
	_ = os.Chmod(tmp, 0o644)
	var renameErr error
	for attempt := 0; attempt < 4; attempt++ {
		renameErr = os.Rename(tmp, targetPath)
		if renameErr == nil {
			return nil
		}
		time.Sleep(time.Duration(attempt+1) * 15 * time.Millisecond)
	}
	cleanup()
	return renameErr
}
