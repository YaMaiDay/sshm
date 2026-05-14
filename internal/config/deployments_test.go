package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveLoadDeployments(t *testing.T) {
	home := t.TempDir()
	file := DeploymentsFile{
		Apps: []DeploymentApp{
			{
				Name:           "api",
				Server:         "prod/api-01",
				Source:         DeploySourceGit,
				Repo:           "git@github.com:owner/api.git",
				Branch:         "main",
				Path:           "/data/api",
				Credential:     DeployCredentialSSH,
				CredentialName: "api-deploy-key",
				WaitSeconds:    15,
				Favorite:       true,
				Pinned:         true,
				PinnedOrder:    9,
				BeforeCommands: []string{"systemctl stop api"},
				AfterCommands:  []string{"systemctl restart api"},
			},
		},
		Records: []DeploymentRecord{
			{App: "api", ServerName: "api-01", Status: DeployStatusSuccess, CurrentVersion: "abc123"},
		},
	}
	if err := SaveDeployments(home, file); err != nil {
		t.Fatal(err)
	}
	got, ok, err := LoadDeployments(home)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected deployments file")
	}
	if len(got.Apps) != 1 || got.Apps[0].Name != "api" || got.Apps[0].Credential != DeployCredentialSSH || got.Apps[0].WaitSeconds != 15 {
		t.Fatalf("apps not loaded: %+v", got.Apps)
	}
	if !got.Apps[0].Favorite || !got.Apps[0].Pinned || got.Apps[0].PinnedOrder != 9 {
		t.Fatalf("favorite/pinned not loaded: %+v", got.Apps[0])
	}
	if len(got.Records) != 1 || got.Records[0].CurrentVersion != "abc123" {
		t.Fatalf("records not loaded: %+v", got.Records)
	}
	info, err := os.Stat(DeploymentsPath(home))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestValidateDeploymentApp(t *testing.T) {
	if err := ValidateDeploymentApp(DeploymentApp{Name: "api", Source: DeploySourceGit, Path: "/data/api"}); err == nil {
		t.Fatal("expected git repo validation error")
	}
	if err := ValidateDeploymentApp(DeploymentApp{Name: "web", Source: DeploySourceRelease, Path: "/data/web", Repo: "owner/web", Version: "v1", Asset: "web.tar.gz"}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDeploymentApp(DeploymentApp{Name: "web", Source: DeploySourceRelease, Path: "/data/web", ReleaseURL: "https://github.com/owner/web/releases/download/v1/web.tar.gz"}); err != nil {
		t.Fatal(err)
	}
}

func TestAppendDeploymentRecordTrimsHistory(t *testing.T) {
	home := t.TempDir()
	if err := SaveDeployments(home, DeploymentsFile{Apps: []DeploymentApp{{Name: "api", Source: DeploySourceGit, Repo: "git@github.com:o/r.git", Path: "/data/api"}}}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < MaxDeploymentRecords+5; i++ {
		if err := AppendDeploymentRecord(home, DeploymentRecord{App: "api", ServerName: "host", CurrentVersion: strings.Repeat("a", 1)}); err != nil {
			t.Fatal(err)
		}
	}
	got, _, err := LoadDeployments(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Records) != MaxDeploymentRecords {
		t.Fatalf("len(records) = %d, want %d", len(got.Records), MaxDeploymentRecords)
	}
	if filepath.Base(DeploymentsPath(home)) != "deployments.toml" {
		t.Fatalf("unexpected path: %s", DeploymentsPath(home))
	}
}
