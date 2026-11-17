package runworkspace

import (
	"fmt"
	"strings"

	"asterlab-mygo-prototype/modules/runs"
)

// blindQueryParams is the exact CLI input JSON for the run.workspace blind
// read. The view value is blind or unblinded; Core only honours blind when
// the frozen design declares a non-empty blinding basis, otherwise it
// rejects with INVALID_INPUT and the UI stays unblinded.
func blindQueryParams(runID string) string {
	return fmt.Sprintf(`{"id":%q,"view":"blind"}`, runID)
}

// blindSwitchLabel is the Run top-bar toggle text. It never shows the raw
// view enum or the run id (UX-P2): the operator sees only the mask state.
func blindSwitchLabel(blind bool) string {
	if blind {
		return "● 设盲视图"
	}
	return "○ 设盲视图"
}

// toggleBlind flips the Run blinding mask. Entering blind confirms through
// Core's run.workspace blind projection via QueryProjection when the host
// wires it: blind mode hides real group treatment names (pure display
// masking; wells stay visible as coordinates) and Core only grants it to a
// design that declares a blinding basis. Leaving blind is a local unmask —
// the groups are already in the cells, so no Core round-trip is needed.
// Every failure keeps the current mask and reports cause + retry (UX-STATE,
// UX-P5); Go never desensitises anything itself.
func (s *State) toggleBlind() {
	if s == nil {
		return
	}
	bench := s.bench()
	if bench.Blind {
		bench.Blind = false
		s.BlindErr = ""
		return
	}
	if s.Callbacks.QueryProjection != nil && s.VM != nil {
		_, err := s.Callbacks.QueryProjection(runs.QueryProjectionInput{
			Kind:       channelQueryWorkspace,
			ParamsJSON: blindQueryParams(s.VM.RunID),
		})
		if err != nil {
			s.BlindErr = blindQueryError(err)
			return
		}
	}
	bench.Blind = true
	s.BlindErr = ""
}

// blindQueryError maps a blind-projection rejection to operator wording
// (UX-P5): a design without a blinding basis is the expected case and says
// so; everything else names the cause and offers a retry.
func blindQueryError(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "INVALID_INPUT"):
		return "该设计未声明盲法依据，无法进入设盲视图"
	case strings.Contains(msg, "NOT_FOUND"):
		return "Run 记录不存在，请刷新后重试"
	default:
		return "设盲视图加载失败（" + msg + "），请重试"
	}
}
