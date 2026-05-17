package transfer

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/YaMaiDay/sshm/internal/actions"
	"github.com/YaMaiDay/sshm/internal/config"
	"github.com/YaMaiDay/sshm/internal/host"
)

type Service struct{}

type EntrySpec struct {
	ID         string
	Time       time.Time
	Kind       string
	Status     string
	Source     string
	TargetDir  string
	IsDir      bool
	TotalBytes int64
}

type JobResult struct {
	Output string
	Err    error
}

type CopyResult struct {
	Output string
	Err    error
}

func BuildEntry(h host.Host, spec EntrySpec) config.TransferEntry {
	at := spec.Time
	if at.IsZero() {
		at = time.Now()
	}
	status := spec.Status
	if strings.TrimSpace(status) == "" {
		status = config.TransferStatusQueued
	}
	id := spec.ID
	if strings.TrimSpace(id) == "" {
		id = config.NewTransferID(at)
	}
	return config.NormalizeTransferEntry(config.TransferEntry{
		ID:           id,
		Time:         at.Format(time.RFC3339),
		Kind:         spec.Kind,
		Status:       status,
		HostCategory: h.Category,
		HostName:     h.Name,
		Host:         h.HostName,
		User:         h.User,
		Port:         h.Port,
		Source:       spec.Source,
		TargetDir:    spec.TargetDir,
		IsDir:        spec.IsDir,
		TotalBytes:   spec.TotalBytes,
		UpdatedAt:    at.Format(time.RFC3339),
	})
}

func (Service) CheckRsync(ctx context.Context, h host.Host) error {
	cmd, cleanup := actions.RemoteRsyncCheckCommand(ctx, h)
	defer cleanup()
	return cmd.Run()
}

func (Service) InstallRsync(ctx context.Context, h host.Host) (string, error) {
	cmd, cleanup := actions.RemoteRsyncInstallCommand(ctx, h)
	defer cleanup()
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func (Service) CommandForEntry(ctx context.Context, h host.Host, entry config.TransferEntry) (*exec.Cmd, actions.Cleanup) {
	if entry.Kind == "download" {
		return actions.RsyncDownloadCommandContext(ctx, h, entry.Source, entry.TargetDir)
	}
	return actions.RsyncUploadCommandContext(ctx, h, entry.Source, entry.TargetDir)
}

func (s Service) RunJob(ctx context.Context, h host.Host, entry config.TransferEntry, onProgress func(string)) JobResult {
	cmd, cleanup := s.CommandForEntry(ctx, h, entry)
	output, err := s.RunRsyncWithProgress(cmd, onProgress)
	cleanup()
	return JobResult{Output: output, Err: err}
}

func (Service) Upload(ctx context.Context, h host.Host, localPath string, remoteDir string, recursive bool) CopyResult {
	cmd, cleanup := actions.SCPUploadCommandContext(ctx, h, localPath, remoteDir, recursive)
	output, err := cmd.CombinedOutput()
	cleanup()
	return CopyResult{Output: string(output), Err: err}
}

func (Service) Download(ctx context.Context, h host.Host, remotePath string, saveDir string, recursive bool) CopyResult {
	cmd, cleanup := actions.SCPDownloadCommandContext(ctx, h, remotePath, saveDir, recursive)
	output, err := cmd.CombinedOutput()
	cleanup()
	return CopyResult{Output: string(output), Err: err}
}

func (Service) RunRsyncWithProgress(cmd *exec.Cmd, onProgress func(string)) (string, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	var mu sync.Mutex
	var output strings.Builder
	lastProgress := ""
	collect := func(text string) {
		progress := ""
		mu.Lock()
		output.WriteString(text)
		if !strings.HasSuffix(text, "\n") {
			output.WriteString("\n")
		}
		if progressText := ProgressText(text); progressText != "" && progressText != lastProgress {
			lastProgress = progressText
			progress = progressText
		}
		mu.Unlock()
		if progress != "" && onProgress != nil {
			onProgress(progress)
		}
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		readRsyncProgress(stdout, collect)
	}()
	go func() {
		defer wg.Done()
		readRsyncProgress(stderr, collect)
	}()
	err = cmd.Wait()
	wg.Wait()
	mu.Lock()
	text := output.String()
	mu.Unlock()
	return text, err
}

func readRsyncProgress(r io.Reader, collect func(string)) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	scanner.Split(splitRsyncProgress)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text != "" {
			collect(text)
		}
	}
}

func splitRsyncProgress(data []byte, atEOF bool) (advance int, token []byte, err error) {
	for i, b := range data {
		if b == '\n' || b == '\r' {
			return i + 1, data[:i], nil
		}
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func RemoteSizeBytes(h host.Host, remotePath string) int64 {
	cmd, cleanup := actions.RemoteSizeCommand(h, remotePath)
	defer cleanup()
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	size, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil || size < 0 {
		return 0
	}
	return size
}

func LocalSizeBytes(path string) int64 {
	info, err := os.Lstat(path)
	if err != nil {
		return 0
	}
	if !info.IsDir() {
		return info.Size()
	}
	var total int64
	_ = filepath.WalkDir(path, func(itemPath string, entry os.DirEntry, err error) error {
		if err != nil || entry == nil {
			return nil
		}
		info, err := entry.Info()
		if err != nil || info.IsDir() {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}
