package agentedit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type Target struct { Kind string `json:"kind"`; ObjectID string `json:"object_id"`; Location json.RawMessage `json:"location,omitempty"` }
type ProposalOrigin struct { Source string `json:"source"`; Rationale *string `json:"rationale,omitempty"` }
type FieldDiff struct { Path string `json:"path"`; Operation string `json:"operation"`; Before any `json:"before,omitempty"`; After any `json:"after,omitempty"` }
type Proposal struct { SchemaVersion uint32 `json:"schema_version"`; ProposalID string `json:"proposal_id"`; Target Target `json:"target"`; BaseVersionID string `json:"base_version_id"`; ExpectedVersion int64 `json:"expected_version"`; Origin ProposalOrigin `json:"origin"`; Diffs []FieldDiff `json:"diffs"` }
type ResolvedFieldDiff struct { Path string `json:"path"`; Operation string `json:"operation"`; Current any `json:"current"`; Requested any `json:"requested"`; Changed bool `json:"changed"` }
type AgentEditIssue struct { Code string `json:"code"`; Severity string `json:"severity"`; Path string `json:"path"`; Message string `json:"message"`; Affects []string `json:"affects"`; NextAction string `json:"next_action"` }
type ValidationResult struct { SchemaVersion uint32 `json:"schema_version"`; ProposalID string `json:"proposal_id"`; ProposalDigest string `json:"proposal_digest"`; Target Target `json:"target"`; CurrentVersionID string `json:"current_version_id"`; CurrentRevision int64 `json:"current_revision"`; Status string `json:"status"`; Diffs []ResolvedFieldDiff `json:"diffs"`; Issues []AgentEditIssue `json:"issues"`; InvalidatedRefs []string `json:"invalidated_refs"`; RequiresConfirmation bool `json:"requires_confirmation"`; ConfirmationPrompt *string `json:"confirmation_prompt,omitempty"` }
type ConfirmRequest struct { SchemaVersion uint32 `json:"schema_version"`; Proposal Proposal `json:"proposal"`; ProposalDigest string `json:"proposal_digest"`; IdempotencyKey string `json:"idempotency_key"`; ConfirmedBy string `json:"confirmed_by"`; Acknowledged []string `json:"acknowledged"`; Note *string `json:"note,omitempty"` }
type CommitResult struct { SchemaVersion uint32 `json:"schema_version"`; ProposalID string `json:"proposal_id"`; ProposalDigest string `json:"proposal_digest"`; Target Target `json:"target"`; PreviousVersionID string `json:"previous_version_id"`; NewVersionID string `json:"new_version_id"`; NewRevision int64 `json:"new_revision"`; Applied []ResolvedFieldDiff `json:"applied"`; InvalidatedRefs []string `json:"invalidated_refs"`; ConfirmedBy string `json:"confirmed_by"`; CommittedAt string `json:"committed_at"`; IdempotencyKey string `json:"idempotency_key"`; Replayed bool `json:"replayed"` }
type AgentEditError struct { Code string `json:"code"`; Message string `json:"message"`; Path *string `json:"path,omitempty"`; CurrentVersion *int64 `json:"current_version,omitempty"`; CurrentVersionID *string `json:"current_version_id,omitempty"`; Issues []AgentEditIssue `json:"issues,omitempty"` }
func (e *AgentEditError) Error() string { if e == nil { return "" }; if e.Message == "" { return e.Code }; return e.Code+": "+e.Message }
type Client interface { Validate(context.Context, Proposal) (ValidationResult, error); Commit(context.Context, ConfirmRequest) (CommitResult, error) }
type CLIClient struct { Executable string; DataDir string }
type envelope struct { OK bool `json:"ok"`; Tool string `json:"tool"`; Error *AgentEditError `json:"error"`; Result json.RawMessage `json:"result"` }
func (c CLIClient) executable() string {
	if c.Executable!="" { return c.Executable }
	if p:=os.Getenv("LABWORKBENCH_CLI"); p!="" { return p }
	if dir,err:=os.Getwd(); err==nil {
		for {
			p:=filepath.Join(dir,"target","debug","labworkbench.exe")
			if _,err:=os.Stat(p); err==nil { return p }
			parent:=filepath.Dir(dir); if parent==dir {break}; dir=parent
		}
	}
	return "labworkbench"
}
func (c CLIClient) run(ctx context.Context, tool string, input any, output any) error { b,err:=json.Marshal(input); if err!=nil{return err}; cmd:=exec.CommandContext(ctx,c.executable(),tool,"--input","-"); dir:=c.DataDir; if dir=="" {dir=os.Getenv("LABWORKBENCH_DATA_DIR")}; if dir!="" {cmd.Env=append(os.Environ(),"LABWORKBENCH_DATA_DIR="+dir)}; cmd.Stdin=bytes.NewReader(b); var stdout,stderr bytes.Buffer; cmd.Stdout=&stdout; cmd.Stderr=&stderr; runErr:=cmd.Run(); var env envelope; if err=json.Unmarshal(stdout.Bytes(),&env); err!=nil { if runErr!=nil { return fmt.Errorf("%s: %w: %s",tool,runErr,stderr.String()) }; return fmt.Errorf("%s returned invalid JSON: %w",tool,err) }; if !env.OK { if env.Error==nil {return errors.New("CLI returned ok=false without error")}; return env.Error }; if runErr!=nil {return fmt.Errorf("%s: %w: %s",tool,runErr,stderr.String())}; if len(env.Result)==0 {env.Result=stdout.Bytes()}; if err=json.Unmarshal(env.Result,output); err!=nil {return fmt.Errorf("decode %s result: %w",tool,err)}; return nil }
func (c CLIClient) Validate(ctx context.Context,p Proposal)(ValidationResult,error){var r ValidationResult; err:=c.run(ctx,"agent_edit.validate",p,&r); return r,err}
func (c CLIClient) Commit(ctx context.Context,r ConfirmRequest)(CommitResult,error){var out CommitResult; err:=c.run(ctx,"agent_edit.commit",r,&out); return out,err}
