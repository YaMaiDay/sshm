package resource

import (
	"context"

	"github.com/YaMaiDay/sshm/internal/actions"
	"github.com/YaMaiDay/sshm/internal/config"
	"github.com/YaMaiDay/sshm/internal/dbmonitor"
	"github.com/YaMaiDay/sshm/internal/execresult"
	"github.com/YaMaiDay/sshm/internal/host"
)

type Kind string

const (
	KindServices   Kind = "services"
	KindContainers Kind = "containers"
	KindPorts      Kind = "ports"
)

type PartResult struct {
	Services   []ServiceDetail
	Containers []ContainerDetail
	Ports      []PortDetail
	ErrText    string
	Err        error
}

type CommandResult = execresult.Result

type ContainerDetailResult struct {
	Detail  ContainerExtraDetail
	ErrText string
	Err     error
}

type ServiceDetailResult struct {
	Detail  ServiceDetail
	ErrText string
	Err     error
}

type ProcessDetailResult struct {
	Detail  ProcessExtraDetail
	ErrText string
	Err     error
}

type DatabaseDetailResult struct {
	Detail  DatabaseExtraDetail
	ErrText string
	Err     error
}

type Service struct{}

func (Service) ExecuteScript(ctx context.Context, h host.Host, script string) CommandResult {
	result, cleanup := actions.RemoteCommandContext(ctx, h, script)
	cleanup()
	return CommandResult{Output: result.Output, Err: result.Err, ExitCode: result.ExitCode}
}

func (Service) FetchPart(ctx context.Context, h host.Host, kind Kind) PartResult {
	switch kind {
	case KindServices:
		result, cleanup := actions.RemoteCommandContext(ctx, h, ServiceListScript())
		cleanup()
		services, errText := ParseServiceDetails(result.Output)
		return PartResult{Services: services, ErrText: errText, Err: result.Err}
	case KindContainers:
		result, cleanup := actions.RemoteCommandContext(ctx, h, ContainerDetailScript())
		cleanup()
		containers, errText := ParseContainerDetails(result.Output)
		return PartResult{Containers: containers, ErrText: errText, Err: result.Err}
	case KindPorts:
		result, cleanup := actions.RemoteCommandContext(ctx, h, PortDetailScript())
		cleanup()
		ports, errText := ParsePortDetails(result.Output)
		return PartResult{Ports: ports, ErrText: errText, Err: result.Err}
	default:
		return PartResult{}
	}
}

func (Service) FetchContainerDetail(ctx context.Context, h host.Host, name string) ContainerDetailResult {
	result, cleanup := actions.RemoteCommandContext(ctx, h, ContainerExtraDetailScript(name))
	cleanup()
	detail, errText := ParseContainerExtraDetail(result.Output)
	return ContainerDetailResult{Detail: detail, ErrText: errText, Err: result.Err}
}

func (Service) FetchServiceDetail(ctx context.Context, h host.Host, name string) ServiceDetailResult {
	result, cleanup := actions.RemoteCommandContext(ctx, h, ServiceExtraDetailScript(name))
	cleanup()
	detail, errText := ParseServiceExtraDetail(result.Output)
	return ServiceDetailResult{Detail: detail, ErrText: errText, Err: result.Err}
}

func (Service) FetchProcessDetail(ctx context.Context, h host.Host, pid string) ProcessDetailResult {
	result, cleanup := actions.RemoteCommandContext(ctx, h, ProcessExtraDetailScript(pid))
	cleanup()
	detail, errText := ParseProcessExtraDetail(result.Output)
	return ProcessDetailResult{Detail: detail, ErrText: errText, Err: result.Err}
}

func (Service) FetchDatabaseDetail(ctx context.Context, h host.Host, item config.ManagedResource, detail DatabaseDetail) DatabaseDetailResult {
	script := dbmonitor.MetricScriptForRuntime(item, dbmonitor.Runtime{Container: detail.Container})
	result, cleanup := actions.RemoteCommandContext(ctx, h, script)
	cleanup()
	parsed, errText := dbmonitor.Parse(result.Output)
	return DatabaseDetailResult{Detail: parsed, ErrText: errText, Err: result.Err}
}
