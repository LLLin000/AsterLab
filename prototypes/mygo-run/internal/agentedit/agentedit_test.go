package agentedit

import (
	"context"
	"strings"
	"testing"

	"asterlab-mygo-prototype/internal/theme"
	"github.com/egoist/mygo/ui"
)

type fakeClient struct { validation ValidationResult; validateErr,commitErr error; commits int }
func (f *fakeClient) Validate(context.Context,Proposal)(ValidationResult,error){return f.validation,f.validateErr}
func (f *fakeClient) Commit(context.Context,ConfirmRequest)(CommitResult,error){f.commits++;return CommitResult{NewVersionID:"v2",NewRevision:2},f.commitErr}
func TestGroupedDiffAndRejectNoCommit(t *testing.T){
	f:=&fakeClient{validation:ValidationResult{Status:"valid",Diffs:[]ResolvedFieldDiff{{Path:"params.volume",Operation:"set",Current:1,Requested:2,Changed:true},{Path:"params.volume",Operation:"set",Current:2,Requested:3,Changed:true}}}}
	s:=&State{Client:f,JSON:`{"schema_version":1,"proposal_id":"p","target":{"kind":"protocol","object_id":"x"},"base_version_id":"v1","expected_version":1,"origin":{"source":"agent"},"diffs":[{"path":"params.volume","operation":"set","before":1,"after":2}]}`}
	s.validate(); if len(s.Validation.Diffs)!=2||s.Validation.Diffs[0].Path!=s.Validation.Diffs[1].Path{t.Fatal("path diffs were not retained for grouping")}
	tt:=ui.NewTester(func(c *ui.Context){c.SetTheme(theme.LightTheme());View(c,s)},900,600);tt.Frame()
	if !tt.HasText("params.volume")||!tt.HasText("before（声明）") {t.Fatalf("diff review absent: %q",tt.Texts())}
	if err:=tt.Click("拒绝");err!=nil{t.Fatal(err)};tt.Frame()
	if f.commits!=0||!s.Closed{t.Fatalf("reject must close without commit: commits=%d closed=%v",f.commits,s.Closed)}
}
func TestBlockedCodesAreSurfaced(t *testing.T){
	validJSON := `{"schema_version":1,"proposal_id":"p1","target":{"kind":"protocol","object_id":"x"},"base_version_id":"v1","expected_version":1,"origin":{"source":"agent"},"diffs":[{"path":"params.volume","operation":"set","before":1,"after":2}]}`
	for _,code:=range []string{"VERSION_CONFLICT","UNIT_MISMATCH","CONFIRMATION_REQUIRED","INVALID_BINDING","STALE_REFERENCE","FORBIDDEN_FIELD","CONFIRMATION_MISMATCH","IDEMPOTENCY_MISMATCH"}{
		t.Run(code,func(t *testing.T){
			f:=&fakeClient{validateErr:&AgentEditError{Code:code,Message:"rejected"}}
			s:=&State{Client:f,JSON:validJSON}
			s.validate()
			if !strings.Contains(s.Error,code){t.Fatalf("error code not surfaced: %q",s.Error)}
			f2:=&fakeClient{validation:ValidationResult{Status:"valid",ProposalDigest:"sha256:abc"},commitErr:&AgentEditError{Code:code,Message:"commit rejected"}}
			s2:=&State{Client:f2,JSON:validJSON}
			s2.validate()
			s2.commit()
			if !strings.Contains(s2.Error,code){t.Fatalf("commit error code not surfaced: %q",s2.Error)}
		})
	}
}
func TestViewRendersWideAndNarrow(t *testing.T){for _,size:=range [][2]int{{1360,880},{900,600}}{for _,dark:=range []bool{false,true}{tt:=ui.NewTesterWith(func(c *ui.Context){s:=&State{};if !dark{c.SetTheme(theme.LightTheme())};View(c,s)},size[0],size[1],ui.TesterOptions{Dark:dark});tt.Frame()}}}
