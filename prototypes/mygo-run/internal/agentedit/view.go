package agentedit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
)

// State retains the editable proposal and its read-only Core assessment.
type State struct {
	Client Client
	JSON string
	Path string
	Proposal *Proposal
	Validation *ValidationResult
	Error string
	Open bool
	Closed bool
	Acknowledged map[string]*bool
	ConfirmedBy string
	Result *CommitResult
	OnCommitted func(CommitResult)
}
func (s *State) validate() {
	s.Validation=nil
	s.Result=nil
	var p Proposal
	if err:=json.Unmarshal([]byte(s.JSON),&p); err!=nil { s.Error="INVALID_PROPOSAL: "+err.Error(); return }
	s.Proposal=&p
	if s.Client==nil { s.Error="校验客户端不可用"; return }
	r,err:=s.Client.Validate(context.Background(),p)
	if err!=nil {s.Error=err.Error();return}
	s.Validation=&r; s.Error=""; s.Acknowledged=map[string]*bool{}
	for _,i:=range r.Issues { if i.Severity=="needs_confirmation" { b:=false; s.Acknowledged[i.Path]=&b } }
}
func (s *State) commit() {
	if s.Client==nil||s.Proposal==nil||s.Validation==nil||s.Validation.Status=="blocked" {return}
	acks:=[]string{}
	for path,b:=range s.Acknowledged {if b!=nil&&*b {acks=append(acks,path)}}
	if s.Validation.RequiresConfirmation { for _,i:=range s.Validation.Issues {if i.Severity=="needs_confirmation"&&(s.Acknowledged[i.Path]==nil||!*s.Acknowledged[i.Path]) {return}} }
	r,err:=s.Client.Commit(context.Background(),ConfirmRequest{SchemaVersion:1,Proposal:*s.Proposal,ProposalDigest:s.Validation.ProposalDigest,IdempotencyKey:s.Proposal.ProposalID,ConfirmedBy:s.ConfirmedBy,Acknowledged:acks})
	if err!=nil {s.Error=err.Error();return}
	s.Error=""; s.Result=&r; if s.OnCommitted!=nil{s.OnCommitted(r)}
}
// View applies UX-P1/P2/P3/P5 and UX-LAYOUT: review stays grouped by editable path and scrolls on narrow windows.
func View(c *ui.Context, s *State) {
	if s == nil || s.Closed {
		return
	}
	t := c.Theme()
	ui.Column(c).Fill().Padding(t.Space(3)).Gap(t.Space(1.5)).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
			ui.Text(c, "Agent 提案 · 草稿").Bold().TextColor(t.Text)
			ui.Spacer(c)
			if components.Button(c, "加载", components.ButtonSecondary).Clicked() {
				if b, e := __readFile(s.Path); e == nil {
					s.JSON = string(b)
					s.Error = ""
				} else {
					s.Error = e.Error()
				}
			}
			if components.Button(c, "校验提案", components.ButtonSecondary).Clicked() {
				s.validate()
			}
		})
		ui.Row(c).FillWidth().Gap(t.Space(1.5)).Children(func() {
			ui.TextInput(c, &s.Path).Placeholder("修改建议文件路径").Grow(1)
			ui.TextInput(c, &s.JSON).Placeholder("粘贴结构化修改建议内容").Grow(1)
		})
		if s.Error != "" {
			ui.Text(c, s.Error).TextColor(t.Danger)
		}
		if s.Validation != nil {
			v := s.Validation
			ui.Text(c, "状态："+v.Status).Bold().TextColor(t.Text)
			if v.ConfirmationPrompt != nil {
				ui.Text(c, *v.ConfirmationPrompt).TextColor(t.TextMuted)
			}
			ui.Scroll(c).Grow(1).MinHeight(120).Children(func() {
				ui.Column(c).FillWidth().Gap(t.Space(1.5)).Children(func() {
					groups := map[string][]ResolvedFieldDiff{}
					order := []string{}
					for _, d := range v.Diffs {
						if _, ok := groups[d.Path]; !ok {
							order = append(order, d.Path)
						}
						groups[d.Path] = append(groups[d.Path], d)
					}
					for _, path := range order {
						path := path
						ui.Column(c).FillWidth().Padding(t.Space(1.5)).Border(1, t.Border).Radius(t.Space(1)).Children(func() {
							ui.Text(c, path).Bold().TextColor(t.Text)
							for _, d := range groups[path] {
								claimed := "(未声明)"
								if s.Proposal != nil {
									for _, p := range s.Proposal.Diffs {
										if p.Path == d.Path && p.Before != nil {
											claimed = formatEditValue(p.Before)
										}
									}
								}
								ui.Text(c, fmt.Sprintf("建议前原值: %s · 当前方案值: %s", claimed, formatEditValue(d.Current))).TextColor(t.TextMuted)
								ui.Text(c, fmt.Sprintf("%s → %s · operation: %s", d.Path, formatEditValue(d.Requested), d.Operation)).TextColor(t.Text)
							}
						})
					}
					for _, i := range v.Issues {
						i := i
						ui.Column(c).FillWidth().Children(func() {
							ui.Text(c, fmt.Sprintf("%s · %s · %s", i.Severity, i.Code, i.Path)).TextColor(t.Danger)
							ui.Text(c, i.Message+" · "+i.NextAction).TextColor(t.TextMuted)
						})
					}
				})
			})
			if v.RequiresConfirmation {
				for _, i := range v.Issues {
					if i.Severity == "needs_confirmation" {
						b := s.Acknowledged[i.Path]
						components.Checkbox(c, b, "确认 "+i.Path)
					}
				}
			}
		}
		ui.Row(c).FillWidth().Wrap().Justify(ui.End).Gap(t.Space(2)).Children(func() {
			if components.Button(c, "拒绝", components.ButtonSecondary).Clicked() {
				s.Open = false
				s.Closed = true
				s.JSON = ""
				s.Proposal = nil
				s.Validation = nil
			}
			can := s.Validation != nil && s.Validation.Status != "blocked"
			if can && s.Validation.RequiresConfirmation {
				for _, i := range s.Validation.Issues {
					if i.Severity == "needs_confirmation" && (s.Acknowledged[i.Path] == nil || !*s.Acknowledged[i.Path]) {
						can = false
					}
				}
			}
			b := components.Button(c, "接受并提交", components.ButtonPrimary)
			b.Disabled(!can)
			if b.Clicked() {
				s.commit()
			}
		})
		if s.Result != nil {
			ui.Text(c, fmt.Sprintf("已保存 · new_version_id: %s · new_revision: %d", s.Result.NewVersionID, s.Result.NewRevision)).TextColor(t.Success)
		}
	})
}
func formatEditValue(v any) string {
	if v == nil {
		return "null"
	}
	if m, ok := v.(map[string]any); ok {
		if val, has := m["value"]; has {
			unit, _ := m["unit"].(string)
			src, _ := m["source"].(string)
			out := fmt.Sprint(val)
			if unit != "" {
				out += " " + unit
			}
			if src != "" {
				out += " (" + src + ")"
			}
			return out
		}
	}
	return fmt.Sprint(v)
}
var __readFile = func(path string)([]byte,error) { if strings.TrimSpace(path)=="" {return nil,fmt.Errorf("请输入文件路径")}; return os.ReadFile(path) }
