package tui

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	transferservice "github.com/YaMaiDay/sshm/internal/transfer"
)

func (m Model) transferProgressText(t activeTransfer) string {
	if t.Kind == "" {
		return ""
	}
	if t.Total <= 0 {
		return fmt.Sprintf(m.t("%s: %s", "%s中：%s"), t.Kind, filepath.Base(t.Source))
	}
	current := int64(0)
	if (t.Kind == "上传" || t.Kind == "Upload") && t.HostIndex >= 0 && t.HostIndex < len(m.states) && t.RemotePath != "" {
		current = transferservice.RemoteSizeBytes(m.states[t.HostIndex].Host, t.RemotePath)
	} else {
		current = transferservice.LocalSizeBytes(t.LocalPath)
	}
	percent := int(float64(current) / float64(t.Total) * 100)
	if percent < 0 {
		percent = 0
	}
	if percent > 99 {
		percent = 99
	}
	return fmt.Sprintf(m.t("%s: %s  %d%%", "%s中：%s  %d%%"), t.Kind, filepath.Base(t.Source), percent)
}

func remoteJoin(dir, name string) string {
	if dir == "" || dir == "/" {
		return "/" + name
	}
	return strings.TrimRight(dir, "/") + "/" + name
}

func transferErrorText(err error, output string) string {
	text := cleanTransferOutput(output)
	if text != "" {
		return text
	}
	if err == nil {
		return "未知错误"
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return fmt.Sprintf("命令退出码 %d", exitErr.ExitCode())
	}
	return err.Error()
}

func cleanTransferOutput(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return ""
	}
	lines := strings.Split(output, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "** WARNING:") ||
			strings.HasPrefix(line, "** This session") ||
			strings.HasPrefix(line, "** The server") {
			continue
		}
		if transferservice.ProgressText(line) != "" {
			continue
		}
		return line
	}
	return ""
}
