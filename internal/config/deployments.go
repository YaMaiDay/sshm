package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pelletier/go-toml/v2"
)

const MaxDeploymentRecords = 100

const (
	DeploySourceGit     = "git"
	DeploySourceRelease = "release"

	DeployFetchLocal  = "local"
	DeployFetchRemote = "remote"

	DeployCredentialNone  = "none"
	DeployCredentialSSH   = "ssh"
	DeployCredentialToken = "token"

	DeployStatusRunning = "running"
	DeployStatusSuccess = "success"
	DeployStatusFailed  = "failed"

	DeployActionDeploy   = "deploy"
	DeployActionRollback = "rollback"
)

var deploymentFileMu sync.Mutex

type DeploymentsFile struct {
	Apps    []DeploymentApp    `toml:"apps"`
	Records []DeploymentRecord `toml:"records"`
}

type DeploymentApp struct {
	Name             string   `toml:"name"`
	Server           string   `toml:"server,omitempty"`
	Source           string   `toml:"source"`
	FetchMode        string   `toml:"fetch_mode,omitempty"`
	Repo             string   `toml:"repo,omitempty"`
	Branch           string   `toml:"branch,omitempty"`
	Version          string   `toml:"version,omitempty"`
	Asset            string   `toml:"asset,omitempty"`
	Path             string   `toml:"path"`
	ReleaseURL       string   `toml:"release_url,omitempty"`
	Credential       string   `toml:"credential,omitempty"`
	CredentialName   string   `toml:"credential_name,omitempty"`
	WaitSeconds      int      `toml:"wait_seconds,omitempty"`
	Favorite         bool     `toml:"favorite,omitempty"`
	Pinned           bool     `toml:"pinned,omitempty"`
	PinnedOrder      int64    `toml:"pinned_order,omitempty"`
	BeforeCommands   []string `toml:"before_commands,omitempty"`
	ResourceCommands []string `toml:"resource_commands,omitempty"`
	UpdateCommands   []string `toml:"update_commands,omitempty"`
	AfterCommands    []string `toml:"after_commands,omitempty"`
	HealthCommands   []string `toml:"health_commands,omitempty"`
	RollbackCommands []string `toml:"rollback_commands,omitempty"`
}

type DeploymentRecord struct {
	ID              string `toml:"id"`
	Time            string `toml:"time"`
	App             string `toml:"app"`
	ServerCategory  string `toml:"server_category"`
	ServerName      string `toml:"server_name"`
	Action          string `toml:"action,omitempty"`
	Source          string `toml:"source"`
	Status          string `toml:"status"`
	PreviousVersion string `toml:"previous_version,omitempty"`
	CurrentVersion  string `toml:"current_version,omitempty"`
	ExitCode        int    `toml:"exit_code"`
	Output          string `toml:"output,omitempty"`
	Error           string `toml:"error,omitempty"`
}

func DeploymentsPath(home string) string {
	return filepath.Join(home, ".config", "sshm", "deployments.toml")
}

func LoadDeployments(home string) (DeploymentsFile, bool, error) {
	deploymentFileMu.Lock()
	defer deploymentFileMu.Unlock()
	return loadDeploymentsUnlocked(home)
}

func loadDeploymentsUnlocked(home string) (DeploymentsFile, bool, error) {
	path := DeploymentsPath(home)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DeploymentsFile{}, false, nil
		}
		return DeploymentsFile{}, false, err
	}
	var file DeploymentsFile
	if err := toml.Unmarshal(data, &file); err != nil {
		return DeploymentsFile{}, true, err
	}
	file.Apps = NormalizeDeploymentApps(file.Apps)
	file.Records = normalizeDeploymentRecords(file.Records)
	return file, true, nil
}

func SaveDeployments(home string, file DeploymentsFile) error {
	deploymentFileMu.Lock()
	defer deploymentFileMu.Unlock()
	return saveDeploymentsUnlocked(home, file)
}

func saveDeploymentsUnlocked(home string, file DeploymentsFile) error {
	path := DeploymentsPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file.Apps = NormalizeDeploymentApps(file.Apps)
	file.Records = normalizeDeploymentRecords(file.Records)
	data, err := toml.Marshal(file)
	if err != nil {
		return err
	}
	return writeFile0600(path, data)
}

func AppendDeploymentRecord(home string, record DeploymentRecord) error {
	deploymentFileMu.Lock()
	defer deploymentFileMu.Unlock()
	file, _, err := loadDeploymentsUnlocked(home)
	if err != nil {
		return err
	}
	record = NormalizeDeploymentRecord(record)
	file.Records = append([]DeploymentRecord{record}, file.Records...)
	return saveDeploymentsUnlocked(home, file)
}

func ValidateDeploymentApp(app DeploymentApp) error {
	if strings.TrimSpace(app.Name) == "" {
		return fmt.Errorf("应用名称不能为空")
	}
	if strings.TrimSpace(app.Path) == "" {
		return fmt.Errorf("项目目录不能为空")
	}
	source := normalizeDeploySource(app.Source)
	switch source {
	case DeploySourceGit:
		if strings.TrimSpace(app.Repo) == "" {
			return fmt.Errorf("Git 仓库不能为空")
		}
	case DeploySourceRelease:
		if strings.TrimSpace(app.ReleaseURL) == "" && (strings.TrimSpace(app.Repo) == "" || strings.TrimSpace(app.Asset) == "") {
			return fmt.Errorf("Release 需要填写下载地址，或填写仓库和资源文件/匹配")
		}
	default:
		return fmt.Errorf("部署来源不支持：%s", app.Source)
	}
	return nil
}

func NormalizeDeploymentApps(apps []DeploymentApp) []DeploymentApp {
	out := make([]DeploymentApp, 0, len(apps))
	for _, app := range apps {
		app.Name = strings.TrimSpace(app.Name)
		app.Server = strings.TrimSpace(app.Server)
		app.Source = normalizeDeploySource(app.Source)
		app.FetchMode = normalizeDeployFetchMode(app.FetchMode)
		app.Repo = strings.TrimSpace(app.Repo)
		app.Branch = strings.TrimSpace(app.Branch)
		app.Version = strings.TrimSpace(app.Version)
		app.Asset = strings.TrimSpace(app.Asset)
		app.Path = strings.TrimSpace(app.Path)
		app.ReleaseURL = strings.TrimSpace(app.ReleaseURL)
		app.Credential = normalizeDeployCredential(app.Credential)
		app.CredentialName = strings.TrimSpace(app.CredentialName)
		if app.WaitSeconds < 0 {
			app.WaitSeconds = 0
		}
		if !app.Pinned {
			app.PinnedOrder = 0
		}
		app.BeforeCommands = normalizeCommandLines(app.BeforeCommands)
		app.ResourceCommands = normalizeCommandLines(app.ResourceCommands)
		app.UpdateCommands = normalizeCommandLines(app.UpdateCommands)
		app.AfterCommands = normalizeCommandLines(app.AfterCommands)
		app.HealthCommands = normalizeCommandLines(app.HealthCommands)
		app.RollbackCommands = normalizeCommandLines(app.RollbackCommands)
		if app.Name == "" || app.Path == "" {
			continue
		}
		out = append(out, app)
	}
	return out
}

func NormalizeDeploymentRecord(record DeploymentRecord) DeploymentRecord {
	now := time.Now().Format(time.RFC3339)
	if strings.TrimSpace(record.ID) == "" {
		record.ID = NewDeploymentID(time.Now())
	}
	if strings.TrimSpace(record.Time) == "" {
		record.Time = now
	}
	record.App = strings.TrimSpace(record.App)
	record.ServerCategory = strings.TrimSpace(record.ServerCategory)
	record.ServerName = strings.TrimSpace(record.ServerName)
	record.Action = normalizeDeployAction(record.Action)
	record.Source = normalizeDeploySource(record.Source)
	record.Status = normalizeDeployStatus(record.Status)
	record.PreviousVersion = strings.TrimSpace(record.PreviousVersion)
	record.CurrentVersion = strings.TrimSpace(record.CurrentVersion)
	record.Output = strings.TrimSpace(record.Output)
	record.Error = strings.TrimSpace(record.Error)
	return record
}

func NewDeploymentID(at time.Time) string {
	return fmt.Sprintf("%d", at.UnixNano())
}

func normalizeDeploymentRecords(records []DeploymentRecord) []DeploymentRecord {
	out := make([]DeploymentRecord, 0, len(records))
	for _, record := range records {
		record = NormalizeDeploymentRecord(record)
		if record.App == "" || record.ServerName == "" {
			continue
		}
		out = append(out, record)
	}
	if len(out) > MaxDeploymentRecords {
		out = out[:MaxDeploymentRecords]
	}
	return out
}

func normalizeDeploySource(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case DeploySourceRelease:
		return DeploySourceRelease
	default:
		return DeploySourceGit
	}
}

func normalizeDeployFetchMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case DeployFetchRemote:
		return DeployFetchRemote
	default:
		return DeployFetchLocal
	}
}

func normalizeDeployCredential(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case DeployCredentialSSH, DeployCredentialToken:
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return DeployCredentialNone
	}
}

func normalizeDeployStatus(status string) string {
	switch strings.TrimSpace(status) {
	case DeployStatusRunning, DeployStatusSuccess, DeployStatusFailed:
		return strings.TrimSpace(status)
	default:
		return DeployStatusSuccess
	}
}

func normalizeDeployAction(action string) string {
	switch strings.TrimSpace(action) {
	case DeployActionRollback:
		return DeployActionRollback
	default:
		return DeployActionDeploy
	}
}

func normalizeCommandLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
