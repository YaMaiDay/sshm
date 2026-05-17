package security

import (
	"context"
	"strings"

	"github.com/YaMaiDay/sshm/internal/actions"
	"github.com/YaMaiDay/sshm/internal/host"
)

type LoginRecordsResult struct {
	Summary       []string
	ErrText       string
	FailedSummary []string
	FailedErrText string
	SSHDSecurity  map[string]string
	SSHDErrText   string
}

type Service struct{}

func (Service) FetchLoginRecords(ctx context.Context, h host.Host) LoginRecordsResult {
	msg := LoginRecordsResult{}
	result, cleanup := actions.RemoteCommandContext(ctx, h, "last -n 100 2>/dev/null || true")
	cleanup()
	if result.Err != nil {
		errText := strings.TrimSpace(result.Output)
		if errText == "" {
			errText = result.Err.Error()
		}
		msg.ErrText = errText
	} else {
		msg.Summary = LoginSummaryRows(ParseLoginRecords(result.Output, 100))
	}

	failedResult, failedCleanup := actions.RemoteCommandContext(ctx, h, FailedLoginScript())
	failedCleanup()
	if strings.TrimSpace(failedResult.Output) != "" {
		msg.FailedSummary, msg.FailedErrText = FailedLoginSummary(failedResult.Output)
	}
	if failedResult.Err != nil && msg.FailedErrText == "" {
		msg.FailedErrText = failedResult.Err.Error()
	}

	sshdResult, sshdCleanup := actions.RemoteCommandContext(ctx, h, SSHDSecurityScript())
	sshdCleanup()
	if strings.TrimSpace(sshdResult.Output) != "" {
		msg.SSHDSecurity = ParseSSHDSettings(sshdResult.Output)
	}
	if sshdResult.Err != nil {
		msg.SSHDErrText = "sshd配置不可读"
	}
	return msg
}
