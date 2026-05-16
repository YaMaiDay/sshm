package tui

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/YaMaiDay/sshm/internal/actions"
	"github.com/YaMaiDay/sshm/internal/config"
	"github.com/YaMaiDay/sshm/internal/host"
	"github.com/YaMaiDay/sshm/internal/monitor"
)

func TestDashboardCardUsesEnglishLabelsByDefault(t *testing.T) {
	m := Model{
		appConfig: config.AppConfig{Language: "en"},
		states: []hostState{{
			Host: host.Host{Name: "api", Category: "aws", HostName: "10.0.0.1", User: "root", Port: "22"},
			Metrics: monitor.Metrics{
				Online:          true,
				CPUPercent:      1,
				CPUCores:        2,
				MemUsed:         1024 * 1024 * 1024,
				MemTotal:        4 * 1024 * 1024 * 1024,
				DiskUsed:        12 * 1024 * 1024 * 1024,
				DiskTotal:       50 * 1024 * 1024 * 1024,
				Load1:           "0.01",
				Load5:           "0.02",
				Load15:          "0.03",
				Uptime:          "up 32 days",
				DockerAvailable: true,
				DockerRunning:   3,
				DockerTotal:     3,
			},
		}},
	}

	out := ansi.Strip(m.renderCard(0, false, 56, false))
	for _, want := range []string{"Mem", "Disk", "Load", "Ctr 0/3/3", "Svc 0", "32d"} {
		if !strings.Contains(out, want) {
			t.Fatalf("rendered card missing %q:\n%s", want, out)
		}
	}
	for _, notWant := range []string{"内存", "磁盘", "负载", "容器", "服务", "风险", "32天"} {
		if strings.Contains(out, notWant) {
			t.Fatalf("rendered card contains Chinese text %q:\n%s", notWant, out)
		}
	}
}

func TestDashboardCardShowsDockerUnavailable(t *testing.T) {
	m := Model{
		appConfig: config.AppConfig{Language: "zh"},
		states: []hostState{{
			Host: host.Host{Name: "api", Category: "aws", HostName: "10.0.0.1", User: "root", Port: "22"},
			Metrics: monitor.Metrics{
				Online: true,
			},
		}},
	}
	out := ansi.Strip(m.renderCard(0, false, 56, false))
	if !strings.Contains(out, "容器 未安装") {
		t.Fatalf("dashboard card should show docker unavailable:\n%s", out)
	}
	if strings.Contains(out, "容器 0") {
		t.Fatalf("dashboard card should not show docker unavailable as zero:\n%s", out)
	}
}

func TestDashboardCardShowsDockerPermissionDenied(t *testing.T) {
	m := Model{
		appConfig: config.AppConfig{Language: "zh"},
		states: []hostState{{
			Host: host.Host{Name: "api", Category: "aws", HostName: "10.0.0.1", User: "root", Port: "22"},
			Metrics: monitor.Metrics{
				Online:       true,
				DockerStatus: "permission",
			},
		}},
	}
	out := ansi.Strip(m.renderCard(0, false, 56, false))
	if !strings.Contains(out, "容器 无权限") {
		t.Fatalf("dashboard card should show docker permission denied:\n%s", out)
	}
	if strings.Contains(out, "容器 未安装") || strings.Contains(out, "容器 0") {
		t.Fatalf("dashboard card should not show docker permission as unavailable/zero:\n%s", out)
	}
}

func TestDashboardCategoryItemsOnlyIncludesRealCategories(t *testing.T) {
	m := Model{
		states: []hostState{
			{Host: host.Host{Name: "web", Category: "prod", Favorite: true, Pinned: true}},
			{Host: host.Host{Name: "db", Category: "prod"}},
			{Host: host.Host{Name: "dev", Category: "dev", Favorite: true, Pinned: true}},
		},
	}

	items := m.dashboardCategoryItems()
	if len(items) < 2 {
		t.Fatalf("dashboardCategoryItems returned %d items, want at least 2", len(items))
	}
	for _, item := range items {
		if item.Kind == "favorite" {
			t.Fatalf("dashboardCategoryItems includes favorite item: %+v", item)
		}
	}
	if items[1].Label != "dev" || items[1].Kind != "category" || items[1].Count != 1 {
		t.Fatalf("first category item = %+v, want dev category count 1", items[1])
	}
}

func TestFilteredIndexesPinnedFirstInCardAll(t *testing.T) {
	m := Model{
		dashboardMode: dashboardCards,
		states: []hostState{
			{Host: host.Host{Name: "a", Category: "aws"}},
			{Host: host.Host{Name: "z", Category: "local", Pinned: true, PinnedOrder: 1}},
			{Host: host.Host{Name: "b", Category: "aws"}},
		},
	}

	indexes := m.filteredIndexes()
	if len(indexes) != 3 {
		t.Fatalf("filteredIndexes length = %d, want 3", len(indexes))
	}
	if got := m.states[indexes[0]].Host.Name; got != "z" {
		t.Fatalf("first host = %q, want global pinned host z", got)
	}
}

func TestFilteredIndexesPinnedFirstInCategoryAll(t *testing.T) {
	m := Model{
		dashboardMode: dashboardCategory,
		states: []hostState{
			{Host: host.Host{Name: "a", Category: "aws"}},
			{Host: host.Host{Name: "z", Category: "local", Pinned: true, PinnedOrder: 1}},
			{Host: host.Host{Name: "b", Category: "aws"}},
		},
	}

	indexes := m.filteredIndexes()
	if len(indexes) != 3 {
		t.Fatalf("filteredIndexes length = %d, want 3", len(indexes))
	}
	if got := m.states[indexes[0]].Host.Name; got != "z" {
		t.Fatalf("first host = %q, want global pinned host z", got)
	}
}

func TestFilteredIndexesGroupedAllKeepsCategoriesBeforePinned(t *testing.T) {
	m := Model{
		dashboardMode: dashboardGrouped,
		states: []hostState{
			{Host: host.Host{Name: "z", Category: "local", Pinned: true, PinnedOrder: 1}},
			{Host: host.Host{Name: "a", Category: "aws"}},
			{Host: host.Host{Name: "b", Category: "aws"}},
		},
	}

	indexes := m.filteredIndexes()
	if len(indexes) != 3 {
		t.Fatalf("filteredIndexes length = %d, want 3", len(indexes))
	}
	if got := m.states[indexes[0]].Host.Category; got != "aws" {
		t.Fatalf("first category = %q, want aws before pinned local", got)
	}
}

func TestFilteredIndexesPinnedOrderNewestFirst(t *testing.T) {
	m := Model{
		dashboardMode: dashboardCards,
		states: []hostState{
			{Host: host.Host{Name: "old", Category: "aws", Pinned: true, PinnedOrder: 1}},
			{Host: host.Host{Name: "new", Category: "aws", Pinned: true, PinnedOrder: 2}},
			{Host: host.Host{Name: "plain", Category: "aws"}},
		},
	}

	indexes := m.filteredIndexes()
	if len(indexes) != 3 {
		t.Fatalf("filteredIndexes length = %d, want 3", len(indexes))
	}
	if got := m.states[indexes[0]].Host.Name; got != "new" {
		t.Fatalf("first host = %q, want newest pinned host new", got)
	}
}

func TestNextPinnedOrder(t *testing.T) {
	hosts := []host.Host{
		{Name: "a", Pinned: true, PinnedOrder: 3},
		{Name: "b", Pinned: true, PinnedOrder: 9},
		{Name: "c"},
	}

	if got := nextPinnedOrder(hosts); got != 10 {
		t.Fatalf("nextPinnedOrder = %d, want 10", got)
	}
}

func TestCycleCategoryClearsFavoriteFilter(t *testing.T) {
	m := Model{
		favoriteOnly: true,
		states: []hostState{
			{Host: host.Host{Name: "a", Category: "prod", Pinned: true, Favorite: true}},
		},
	}

	m.cycleCategory()

	if m.favoriteOnly {
		t.Fatal("favoriteOnly = true, want false")
	}
}

func TestDashboardHostDisplayNameMarksPinnedAndFavorite(t *testing.T) {
	h := host.Host{Name: "web", Category: "prod", Pinned: true, Favorite: true}

	got := dashboardHostDisplayName(h)
	want := "▲ ★ [prod] web"
	if got != want {
		t.Fatalf("dashboardHostDisplayName = %q, want %q", got, want)
	}
}

func TestBuildDeploymentScriptGitIncludesPipeline(t *testing.T) {
	script := buildDeploymentScript(config.DeploymentApp{
		Name:           "api",
		Source:         config.DeploySourceGit,
		Repo:           "git@github.com:owner/api.git",
		Branch:         "main",
		Path:           "/data/api",
		Credential:     config.DeployCredentialSSH,
		CredentialName: "/home/deploy/.ssh/api_deploy_key",
		BeforeCommands: []string{"systemctl stop api"},
		UpdateCommands: []string{"go build ./cmd/api"},
		AfterCommands:  []string{"systemctl restart api"},
		HealthCommands: []string{"curl -fsS http://127.0.0.1:8080/health"},
	}, false)

	for _, want := range []string{
		"== 更新前 ==",
		"== 获取资源 ==",
		"== 更新 ==",
		"== 更新后 ==",
		"== 健康检查 ==",
		"export GIT_SSH_COMMAND=",
		"/home/deploy/.ssh/api_deploy_key",
		"IdentitiesOnly=yes",
		"git clone --branch 'main' 'git@github.com:owner/api.git' '/data/api'",
		"git pull --ff-only",
		"systemctl stop api",
		"go build ./cmd/api",
		"systemctl restart api",
		"curl -fsS http://127.0.0.1:8080/health",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q:\n%s", want, script)
		}
	}
}

func TestDeploymentOutputLinesRendersStageTitles(t *testing.T) {
	m := Model{appConfig: config.AppConfig{Language: "zh"}}
	lines := m.deploymentOutputLines(strings.Join([]string{
		"SSHM_PREVIOUS_VERSION=",
		"== 获取资源 ==",
		"Cloning into '.'...",
		"fatal: Could not read from remote repository.",
		"== 健康检查 ==",
		"ok",
	}, "\n"), 96)
	got := strings.Join(lines, "\n")
	if !strings.Contains(got, "获取资源") || !strings.Contains(got, "健康检查") {
		t.Fatalf("output lines missing stage titles:\n%s", got)
	}
	if strings.Contains(got, "== 获取资源 ==") {
		t.Fatalf("output lines should render stage markers, not show raw marker:\n%s", got)
	}
	if strings.Contains(got, "SSHM_PREVIOUS_VERSION=") {
		t.Fatalf("output lines should hide internal version markers:\n%s", got)
	}
	if !strings.Contains(got, "fatal: Could not read from remote repository.") {
		t.Fatalf("output lines missing command output:\n%s", got)
	}
}

func TestDeploymentOutputShowsInteractiveStages(t *testing.T) {
	app := config.DeploymentApp{
		Name:           "api",
		Source:         config.DeploySourceGit,
		FetchMode:      config.DeployFetchRemote,
		Repo:           "git@github.com:owner/api.git",
		Path:           "/data/api",
		BeforeCommands: []string{"systemctl stop api"},
		UpdateCommands: []string{"make build"},
	}
	m := Model{
		width:  100,
		height: 30,
		activeDeployment: activeDeployment{
			App:      app,
			Action:   config.DeployActionDeploy,
			Output:   "== 更新前 ==\nok\n== 获取资源 ==\nCloning into '.'...\n",
			Running:  true,
			ExitCode: 0,
		},
	}
	view := m.renderDeploymentOutput()
	for _, want := range []string{"✓ Before", "▶ Fetch", "· Update", "Cloning into '.'..."} {
		if !strings.Contains(view, want) {
			t.Fatalf("deployment output missing %q:\n%s", want, view)
		}
	}
}

func TestBuildDeploymentScriptReleaseIncludesDownloadAndUnpack(t *testing.T) {
	script := buildDeploymentScript(config.DeploymentApp{
		Name:           "web",
		Source:         config.DeploySourceRelease,
		Repo:           "owner/web",
		Version:        "v1.2.3",
		Asset:          "web.tar.gz",
		Path:           "/data/web",
		Credential:     config.DeployCredentialToken,
		CredentialName: "GH_RELEASE_TOKEN",
	}, false)

	for _, want := range []string{
		"if [ -n \"${GH_RELEASE_TOKEN:-}\" ]; then",
		"SSHM_GITHUB_AUTH_HEADER=\"Authorization: Bearer ${GH_RELEASE_TOKEN}\"",
		"curl -fL -H \"$SSHM_GITHUB_AUTH_HEADER\" 'https://github.com/owner/web/releases/download/v1.2.3/web.tar.gz'",
		"tar -xzf 'packages/web.tar.gz' -C 'releases/v1.2.3'",
		"ln -sfn 'releases/v1.2.3' current",
		"SSHM_CURRENT_VERSION='v1.2.3'",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q:\n%s", want, script)
		}
	}
}

func TestBuildDeploymentScriptReleaseLatestFixedAsset(t *testing.T) {
	script := buildDeploymentScript(config.DeploymentApp{
		Name:   "web",
		Source: config.DeploySourceRelease,
		Repo:   "owner/web",
		Asset:  "web.tar.gz",
		Path:   "/data/web",
	}, false)

	for _, want := range []string{
		"curl -fL 'https://github.com/owner/web/releases/latest/download/web.tar.gz'",
		"tar -xzf 'packages/web.tar.gz' -C 'releases/latest'",
		"ln -sfn 'releases/latest' current",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q:\n%s", want, script)
		}
	}
}

func TestBuildDeploymentScriptReleasePatternUsesGitHubAPI(t *testing.T) {
	script := buildDeploymentScript(config.DeploymentApp{
		Name:    "kernel",
		Source:  config.DeploySourceRelease,
		Repo:    "owner/kernel",
		Version: "latest",
		Asset:   "freedex-trade-kernel-amd64-*",
		Path:    "/data/kernel",
	}, false)

	for _, want := range []string{
		"SSHM_RELEASE_API='https://api.github.com/repos/owner/kernel/releases/latest'",
		"\"browser_download_url\"",
		"case \"$name\" in 'freedex-trade-kernel-amd64-'*)",
		"未找到匹配的 Release 资源：freedex-trade-kernel-amd64-*",
		"SSHM_RELEASE_ASSET=${SSHM_RELEASE_URL##*/}",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q:\n%s", want, script)
		}
	}
}

func TestDeploymentViewsFitWidth(t *testing.T) {
	app := config.DeploymentApp{
		Name:           "api",
		Server:         "prod/web",
		Source:         config.DeploySourceGit,
		Repo:           "git@github.com:owner/api.git",
		Branch:         "main",
		Path:           "/data/apps/api",
		Credential:     config.DeployCredentialSSH,
		CredentialName: "api-deploy-key",
		BeforeCommands: []string{"systemctl stop api"},
		UpdateCommands: []string{"go build -o api ./cmd/api"},
		AfterCommands:  []string{"systemctl restart api"},
		HealthCommands: []string{"curl -fsS http://127.0.0.1:8080/health"},
	}
	m := Model{
		width:          96,
		height:         28,
		states:         []hostState{{Host: host.Host{Name: "web", Category: "prod", HostName: "10.0.0.10", User: "root", Port: "22"}}},
		deploymentFile: config.DeploymentsFile{Apps: []config.DeploymentApp{app}},
		activeDeployment: activeDeployment{
			HostIndex: 0,
			App:       app,
			Output:    "ok",
		},
		deploymentConfirm: app,
		deploymentForm:    deploymentFormFromApp(app),
	}
	m.deploymentItems = m.deploymentListItems()

	views := []string{
		m.renderDeploymentList(),
		m.renderDeploymentEdit(),
		m.renderDeploymentConfirm(),
		m.renderDeploymentOutput(),
	}
	m.deploymentField = 15
	m.deploymentCursor = len([]rune(m.deploymentForm.UpdateCommands))
	views = append(views, m.renderDeploymentEdit())
	for _, view := range views {
		if got := blockLineCount(view); got > m.height {
			t.Fatalf("deployment view height = %d, want <= %d\n%s", got, m.height, view)
		}
		for _, line := range strings.Split(view, "\n") {
			if got := ansi.StringWidth(line); got > m.width {
				t.Fatalf("deployment view line width = %d, want <= %d\n%s", got, m.width, line)
			}
		}
	}
}

func TestDeploymentListEnterOpensConfirm(t *testing.T) {
	app := config.DeploymentApp{
		Name:   "api",
		Server: "prod/web",
		Source: config.DeploySourceGit,
		Repo:   "git@github.com:owner/api.git",
		Branch: "main",
		Path:   "/data/api",
	}
	m := Model{
		home:             t.TempDir(),
		mode:             modeDeploymentList,
		states:           []hostState{{Host: host.Host{Name: "web", Category: "prod"}}},
		activeDeployment: activeDeployment{HostIndex: 0},
		deploymentFile:   config.DeploymentsFile{Apps: []config.DeploymentApp{app}},
	}
	m.deploymentItems = m.deploymentListItems()
	next, _ := m.updateDeploymentList(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(Model)
	if got.mode != modeDeploymentConfirm {
		t.Fatalf("mode = %v, want modeDeploymentConfirm; status=%q", got.mode, got.status)
	}
}

func TestDeploymentConfirmDoesNotShowResourceCommands(t *testing.T) {
	app := config.DeploymentApp{
		Name:             "api",
		Server:           "prod/web",
		Source:           config.DeploySourceGit,
		Repo:             "git@github.com:owner/api.git",
		Branch:           "main",
		Path:             "/data/api",
		ResourceCommands: []string{"git pull --ff-only"},
	}
	m := Model{
		width:             96,
		height:            24,
		states:            []hostState{{Host: host.Host{Name: "web", Category: "prod"}}},
		deploymentConfirm: app,
	}
	view := m.renderDeploymentConfirm()
	if !strings.Contains(view, "Deployment Info") || strings.Contains(view, "Deployment Queue") || strings.Contains(view, "01") {
		t.Fatalf("single confirm should show deployment info, not queue row:\n%s", view)
	}
	if strings.Contains(view, "Fetch commands") || strings.Contains(view, "git pull --ff-only") {
		t.Fatalf("confirm view should not show resource command detail:\n%s", view)
	}
	if !strings.Contains(view, "Fetch") || !strings.Contains(view, "1 steps") {
		t.Fatalf("confirm view should still show resource step summary:\n%s", view)
	}
}

func TestDeploymentConfirmShowsFiftyHistoryRows(t *testing.T) {
	app := config.DeploymentApp{
		Name:   "api",
		Server: "prod/web",
		Source: config.DeploySourceGit,
		Repo:   "git@github.com:owner/api.git",
		Branch: "main",
		Path:   "/data/api",
	}
	records := []config.DeploymentRecord{}
	for i := 0; i < 52; i++ {
		records = append(records, config.DeploymentRecord{
			Time:            time.Date(2026, 5, 15, 14, i, 0, 0, time.Local).Format(time.RFC3339),
			App:             "api",
			ServerCategory:  "prod",
			ServerName:      "web",
			Action:          config.DeployActionDeploy,
			Status:          config.DeployStatusSuccess,
			PreviousVersion: "old",
			CurrentVersion:  fmt.Sprintf("new-%02d", i),
		})
	}
	m := Model{
		width:             120,
		height:            80,
		states:            []hostState{{Host: host.Host{Name: "web", Category: "prod"}}},
		deploymentFile:    config.DeploymentsFile{Apps: []config.DeploymentApp{app}, Records: records},
		deploymentConfirm: app,
	}
	view := m.renderDeploymentConfirm()
	if !strings.Contains(view, "History 50 records") {
		t.Fatalf("confirm view missing history title:\n%s", view)
	}
	if got := strings.Count(view, "Deploy success"); got != 50 {
		t.Fatalf("history rows = %d, want 50\n%s", got, view)
	}
	if strings.Contains(view, "new-50") || strings.Contains(view, "new-51") {
		t.Fatalf("confirm view should only show first 50 matching history rows:\n%s", view)
	}
}

func TestDeploymentListFiltersByCategory(t *testing.T) {
	apps := []config.DeploymentApp{
		{Name: "api", Server: "prod/web", Source: config.DeploySourceGit, Repo: "git@github.com:owner/api.git", Branch: "main", Path: "/data/api"},
		{Name: "worker", Server: "dev/worker", Source: config.DeploySourceGit, Repo: "git@github.com:owner/worker.git", Branch: "main", Path: "/data/worker"},
	}
	m := Model{
		category:           "prod",
		states:             []hostState{{Host: host.Host{Name: "web", Category: "prod"}}, {Host: host.Host{Name: "worker", Category: "dev"}}},
		deploymentFile:     config.DeploymentsFile{Apps: apps},
		deploymentCategory: "prod",
	}
	m.deploymentItems = m.deploymentListItems()
	if len(m.deploymentItems) != 1 || m.deploymentItems[0].App.Name != "api" {
		t.Fatalf("prod deployment items = %+v", m.deploymentItems)
	}
	m.cycleDeploymentCategory(1)
	if m.deploymentCategory != "" {
		t.Fatalf("category = %q, want all", m.deploymentCategory)
	}
	if len(m.deploymentItems) != 2 {
		t.Fatalf("all deployment items = %+v", m.deploymentItems)
	}
	m.cycleDeploymentCategory(1)
	if m.deploymentCategory != "dev" {
		t.Fatalf("category = %q, want dev", m.deploymentCategory)
	}
	if len(m.deploymentItems) != 1 || m.deploymentItems[0].App.Name != "worker" {
		t.Fatalf("dev deployment items = %+v", m.deploymentItems)
	}
}

func TestDeploymentCategorySkipsEmptyServerCategories(t *testing.T) {
	apps := []config.DeploymentApp{
		{Name: "api", Server: "prod/web", Source: config.DeploySourceGit, Repo: "git@github.com:owner/api.git", Branch: "main", Path: "/data/api"},
	}
	m := Model{
		states:         []hostState{{Host: host.Host{Name: "web", Category: "prod"}}, {Host: host.Host{Name: "empty", Category: "dev"}}},
		deploymentFile: config.DeploymentsFile{Apps: apps},
	}
	m.deploymentItems = m.deploymentListItems()
	m.cycleDeploymentCategory(1)
	if m.deploymentCategory != "prod" {
		t.Fatalf("category = %q, want prod", m.deploymentCategory)
	}
	m.cycleDeploymentCategory(1)
	if m.deploymentCategory != "" {
		t.Fatalf("category = %q, want all", m.deploymentCategory)
	}
}

func TestDeploymentListSortsPinnedAndFiltersFavorites(t *testing.T) {
	apps := []config.DeploymentApp{
		{Name: "normal", Server: "prod/web", Source: config.DeploySourceGit, Repo: "git@github.com:owner/normal.git", Path: "/data/normal"},
		{Name: "favorite", Server: "prod/web", Source: config.DeploySourceGit, Repo: "git@github.com:owner/favorite.git", Path: "/data/favorite", Favorite: true},
		{Name: "pinned", Server: "prod/web", Source: config.DeploySourceGit, Repo: "git@github.com:owner/pinned.git", Path: "/data/pinned", Pinned: true, PinnedOrder: 1},
	}
	m := Model{deploymentFile: config.DeploymentsFile{Apps: apps}}
	items := m.deploymentListItems()
	if got := []string{items[0].App.Name, items[1].App.Name, items[2].App.Name}; !reflect.DeepEqual(got, []string{"pinned", "normal", "favorite"}) {
		t.Fatalf("deployment order = %+v", got)
	}
	m.deploymentFavoriteOnly = true
	items = m.deploymentListItems()
	if len(items) != 1 || items[0].App.Name != "favorite" {
		t.Fatalf("favorite deployment items = %+v", items)
	}
}

func TestDeploymentSelectionKeepsQueueOrder(t *testing.T) {
	apps := []config.DeploymentApp{
		{Name: "api", Server: "prod/api", Source: config.DeploySourceGit, Repo: "git@github.com:owner/api.git", Path: "/data/api"},
		{Name: "web", Server: "prod/web", Source: config.DeploySourceGit, Repo: "git@github.com:owner/web.git", Path: "/data/web", WaitSeconds: 3},
		{Name: "worker", Server: "prod/worker", Source: config.DeploySourceGit, Repo: "git@github.com:owner/worker.git", Path: "/data/worker"},
	}
	m := Model{
		deploymentFile:     config.DeploymentsFile{Apps: apps},
		deploymentSelected: []int{1, 0},
	}
	queue := m.selectedDeploymentQueue()
	if len(queue) != 2 || queue[0].Name != "web" || queue[1].Name != "api" {
		t.Fatalf("queue = %+v", queue)
	}
	if queue[0].WaitSeconds != 3 {
		t.Fatalf("wait seconds = %d, want 3", queue[0].WaitSeconds)
	}
}

func TestDeploymentListEnterClearsPreviousQueueState(t *testing.T) {
	apps := []config.DeploymentApp{
		{Name: "web", Server: "prod/web", Source: config.DeploySourceGit, Repo: "git@github.com:owner/web.git", Path: "/data/web"},
	}
	m := Model{
		mode:               modeDeploymentList,
		states:             []hostState{{Host: host.Host{Name: "web", Category: "prod"}}},
		deploymentFile:     config.DeploymentsFile{Apps: apps},
		activeDeployment:   activeDeployment{HostIndex: 0, Queue: []config.DeploymentApp{{Name: "old"}, {Name: "stale"}}, QueueIndex: 1, QueueFailed: 1, Output: "old output", ExitCode: 1},
		deploymentSelected: []int{0},
	}
	m.deploymentItems = m.deploymentListItems()
	next, _ := m.updateDeploymentList(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(Model)
	if len(got.activeDeployment.Queue) != 0 || got.activeDeployment.Output != "" || got.activeDeployment.QueueFailed != 0 {
		t.Fatalf("active deployment was not reset: %+v", got.activeDeployment)
	}
	view := got.renderDeploymentConfirm()
	if strings.Contains(view, "old output") || strings.Contains(view, "stale") {
		t.Fatalf("confirm view leaked stale queue state:\n%s", view)
	}
}

func TestDeploymentConfirmShowsQueueAndWait(t *testing.T) {
	apps := []config.DeploymentApp{
		{Name: "web", Server: "prod/web", Source: config.DeploySourceGit, Repo: "git@github.com:owner/web.git", Branch: "main", Path: "/data/web", WaitSeconds: 3, ResourceCommands: []string{"git pull"}, UpdateCommands: []string{"make build"}},
		{Name: "api", Server: "prod/api", Source: config.DeploySourceGit, Repo: "git@github.com:owner/api.git", Branch: "main", Path: "/data/api", ResourceCommands: []string{"git pull"}, HealthCommands: []string{"curl -f localhost"}},
	}
	m := Model{
		width:                  110,
		height:                 30,
		deploymentConfirm:      apps[0],
		deploymentConfirmQueue: apps,
	}
	view := m.renderDeploymentConfirm()
	for _, want := range []string{"Deployment Queue", "01", "web", "Wait 3s", "02", "api", "Current Flow", "Fetch 1 steps", "Update 1 steps"} {
		if !strings.Contains(view, want) {
			t.Fatalf("confirm view missing %q:\n%s", want, view)
		}
	}
	for _, notWant := range []string{"History ", "Server    prod/web", "App       web", "    Flow", "Health 1 steps"} {
		if strings.Contains(view, notWant) {
			t.Fatalf("batch confirm view should not show single-app detail %q:\n%s", notWant, view)
		}
	}
}

func TestDeploymentQueueConfirmShowsCurrentFlowAndStopsOnFailure(t *testing.T) {
	apps := []config.DeploymentApp{
		{Name: "web", Server: "prod/web", Source: config.DeploySourceGit, Repo: "git@github.com:owner/web.git", Branch: "main", Path: "/data/web", ResourceCommands: []string{"git pull"}},
		{Name: "api", Server: "prod/api", Source: config.DeploySourceGit, Repo: "git@github.com:owner/api.git", Branch: "main", Path: "/data/api", ResourceCommands: []string{"git pull"}},
	}
	m := Model{
		width:                  110,
		height:                 34,
		deploymentConfirm:      apps[0],
		deploymentConfirmQueue: apps,
		activeDeployment: activeDeployment{
			App:         apps[1],
			Action:      config.DeployActionDeploy,
			Output:      "== 获取资源 ==\nfatal: repository not found\n",
			ExitCode:    128,
			Queue:       apps,
			QueueIndex:  1,
			QueueFailed: 1,
		},
	}
	view := m.renderDeploymentConfirm()
	for _, want := range []string{"✓ 01", "✕ 02", "Current Flow", "Output", "✕ Fetch", "fatal: repository not found", "Exit code 128", "Retry failed r", "Redeploy a"} {
		if !strings.Contains(view, want) {
			t.Fatalf("queue confirm view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "03") {
		t.Fatalf("unexpected queue row:\n%s", view)
	}
}

func TestDeploymentQueueItemStatusChangesWithExecution(t *testing.T) {
	apps := []config.DeploymentApp{
		{Name: "web", Server: "prod/web"},
		{Name: "api", Server: "prod/api"},
	}
	active := activeDeployment{Queue: apps, QueueIndex: 1, Running: true}
	if got := deploymentQueueItemStatus(active, 0); got != "done" {
		t.Fatalf("status[0] = %q, want done", got)
	}
	if got := deploymentQueueItemStatus(active, 1); got != "running" {
		t.Fatalf("status[1] = %q, want running", got)
	}
	active.Running = false
	active.QueueFailed = 1
	active.ExitCode = 1
	if got := deploymentQueueItemStatus(active, 1); got != "failed" {
		t.Fatalf("status[1] = %q, want failed", got)
	}
}

func TestDeploymentListShowsLastRecordActionAndTime(t *testing.T) {
	app := config.DeploymentApp{
		Name:   "api",
		Server: "prod/web",
		Source: config.DeploySourceGit,
		Repo:   "git@github.com:owner/api.git",
		Branch: "main",
		Path:   "/data/api",
	}
	m := Model{
		deploymentFile: config.DeploymentsFile{
			Apps: []config.DeploymentApp{app},
			Records: []config.DeploymentRecord{
				{
					Time:           time.Now().Add(-5 * time.Minute).Format(time.RFC3339),
					App:            "api",
					ServerCategory: "prod",
					ServerName:     "web",
					Action:         config.DeployActionRollback,
					Status:         config.DeployStatusFailed,
				},
			},
		},
	}
	got := m.deploymentLastRecordText(app)
	if !strings.Contains(got, "Rollback failed") || !strings.Contains(got, "m ago") {
		t.Fatalf("last record text = %q", got)
	}
}

func TestDeploymentEditShowsValidationStatus(t *testing.T) {
	m := Model{
		width:             96,
		height:            28,
		mode:              modeDeploymentEdit,
		status:            "保存失败：应用名称不能为空",
		deploymentForm:    deploymentFormFromApp(config.DeploymentApp{Source: config.DeploySourceGit, Credential: config.DeployCredentialNone}),
		activeDeployment:  activeDeployment{HostIndex: 0},
		deploymentEditing: false,
	}
	view := m.renderDeploymentEdit()
	if !strings.Contains(view, "保存失败：应用名称不能为空") {
		t.Fatalf("deployment edit view missing validation status:\n%s", view)
	}
}

func TestDeploymentEditInputWidthsAlign(t *testing.T) {
	app := config.DeploymentApp{
		Source:         config.DeploySourceGit,
		FetchMode:      config.DeployFetchLocal,
		Credential:     config.DeployCredentialSSH,
		Name:           "api",
		Repo:           "git@github.com:owner/repo.git",
		Branch:         "main",
		Path:           "/opt/app",
		CredentialName: "本地或目标服务器私钥路径",
	}
	m := Model{
		width:          120,
		height:         32,
		deploymentForm: deploymentFormFromApp(app),
	}
	view := m.renderDeploymentEdit()
	wantWidth := deploymentInputWidth() + 2
	for _, label := range []string{"App name", "Repo", "Branch", "App dir", "Cred param", "Wait"} {
		line := findLineContaining(view, label)
		if line == "" {
			t.Fatalf("deployment edit view missing %q:\n%s", label, view)
		}
		plain := ansi.Strip(line)
		start := strings.Index(plain, "[")
		end := strings.LastIndex(plain, "]")
		if start < 0 || end <= start {
			t.Fatalf("line %q does not contain input brackets", plain)
		}
		gotWidth := ansi.StringWidth(plain[start : end+1])
		if gotWidth != wantWidth {
			t.Fatalf("%s input width = %d, want %d; line=%q", label, gotWidth, wantWidth, plain)
		}
	}
}

func TestDeploymentDeleteRequiresConfirmation(t *testing.T) {
	home := t.TempDir()
	apps := []config.DeploymentApp{
		{Name: "api", Server: "prod/api", Source: config.DeploySourceGit, Repo: "git@github.com:owner/api.git", Path: "/data/api"},
		{Name: "web", Server: "prod/web", Source: config.DeploySourceGit, Repo: "git@github.com:owner/web.git", Path: "/data/web"},
	}
	if err := config.SaveDeployments(home, config.DeploymentsFile{Apps: apps}); err != nil {
		t.Fatalf("save deployments: %v", err)
	}
	m := Model{
		home:             home,
		mode:             modeDeploymentList,
		deploymentFile:   config.DeploymentsFile{Apps: apps},
		activeDeployment: activeDeployment{HostIndex: 0},
	}
	m.deploymentItems = m.deploymentListItems()

	next, _ := m.updateDeploymentList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	got := next.(Model)
	if got.mode != modeConfirmAction || got.confirm.Kind != confirmDeleteDeployment {
		t.Fatalf("mode=%v confirm=%+v, want delete confirmation", got.mode, got.confirm)
	}
	file, _, err := config.LoadDeployments(home)
	if err != nil {
		t.Fatalf("load deployments: %v", err)
	}
	if len(file.Apps) != 2 {
		t.Fatalf("apps deleted before confirmation: %+v", file.Apps)
	}

	next, _ = got.updateConfirmAction(tea.KeyMsg{Type: tea.KeyEnter})
	got = next.(Model)
	file, _, err = config.LoadDeployments(home)
	if err != nil {
		t.Fatalf("load deployments after delete: %v", err)
	}
	if got.mode != modeDeploymentList {
		t.Fatalf("mode=%v, want deployment list", got.mode)
	}
	if len(file.Apps) != 1 || file.Apps[0].Name != "web" {
		t.Fatalf("apps after confirmed delete = %+v, want only web", file.Apps)
	}
}

func TestHandleDeploymentDoneStopsQueueOnFailure(t *testing.T) {
	home := t.TempDir()
	apps := []config.DeploymentApp{
		{Name: "api", Server: "prod/api", Source: config.DeploySourceGit, Repo: "git@github.com:owner/api.git", Path: "/data/api"},
		{Name: "web", Server: "prod/web", Source: config.DeploySourceGit, Repo: "git@github.com:owner/web.git", Path: "/data/web"},
	}
	if err := config.SaveDeployments(home, config.DeploymentsFile{Apps: apps}); err != nil {
		t.Fatalf("save deployments: %v", err)
	}
	m := Model{
		home:   home,
		states: []hostState{{Host: host.Host{Name: "api", Category: "prod"}}},
		activeDeployment: activeDeployment{
			HostIndex:       0,
			App:             apps[0],
			Action:          config.DeployActionDeploy,
			Queue:           apps,
			QueueIndex:      0,
			Running:         true,
			ProgressID:      "run-1",
			PreviousVersion: "old",
		},
	}

	next, cmd := m.handleDeploymentDone(deploymentDoneMsg{
		ID:     "run-1",
		Result: actions.CommandResult{Err: errors.New("git failed"), ExitCode: 128, Output: "fatal"},
	})
	got := next.(Model)
	if cmd != nil {
		t.Fatal("failed queue should not schedule next deployment")
	}
	if got.activeDeployment.QueueFailed != 0 || got.activeDeployment.Running {
		t.Fatalf("active deployment after failure = %+v", got.activeDeployment)
	}
	if !strings.Contains(got.status, "Deployment queue stopped") {
		t.Fatalf("status = %q, want queue stopped", got.status)
	}
}

func TestContainerDetailRowsShowRawStatus(t *testing.T) {
	m := Model{width: 120}
	rows := containerDetailItemRows(m, containerDetail{
		Name:   "kafka",
		Image:  "apache/kafka:3.9.0",
		Status: "Up 2 weeks (unhealthy)",
		Ports:  "9092/tcp",
	}, 10, 1)
	got := strings.Join(rows, "\n")
	if strings.Contains(got, "Unhealthy 2w") || !strings.Contains(got, "Status Up 2 weeks (unhealthy)") {
		t.Fatalf("container rows should show raw status only:\n%s", got)
	}
}

func TestParseServiceDetailsSortsFailedFirst(t *testing.T) {
	out := strings.Join([]string{
		"nginx.service loaded active running A high performance web server",
		"redis.service loaded failed failed Redis server",
		"cron.service loaded active exited Regular background program processing daemon",
		"old.service loaded inactive dead Old service",
	}, "\n")
	services, errText := parseServiceDetails(out)
	if errText != "" {
		t.Fatalf("errText = %q", errText)
	}
	if len(services) != 4 {
		t.Fatalf("services = %#v", services)
	}
	if services[0].Unit != "redis.service" || serviceDetailKind(services[0]) != "failed" {
		t.Fatalf("first service = %+v, want failed redis first", services[0])
	}
}

func TestParseServiceDetailsWithDiscoveryMetadata(t *testing.T) {
	out := "__SSHM_SERVICE__\tapi.service\tloaded\tactive\trunning\tAPI Service\t/etc/systemd/system/api.service\t/data/api\t/data/api/server\t123\t0\t86016\tFri 2026-05-15 10:00:00 UTC\tFri 2026-05-14 10:00:00 UTC\tFri 2026-05-15 10:00:00 UTC\tFri 2026-05-15 10:00:01 UTC\tFri 2026-05-14 10:00:00 UTC\tenabled\tsuccess\t0\t2\t6\t/system.slice/api.service\tsystem.slice\tapp\tapp\talways\t5000000\t/bin/stop\t/bin/reload\t/etc/systemd/system/api.service.d/override.conf"
	services, errText := parseServiceDetails(out)
	if errText != "" {
		t.Fatalf("errText = %q", errText)
	}
	if len(services) != 1 {
		t.Fatalf("services = %#v, want 1", services)
	}
	item := services[0]
	if item.Unit != "api.service" || item.FragmentPath != "/etc/systemd/system/api.service" || item.WorkingDirectory != "/data/api" || item.ExecStart != "/data/api/server" {
		t.Fatalf("service metadata = %+v", item)
	}
	if item.MainPID != "123" || item.ExecMainPID != "" || item.MemoryCurrent != 86016 || item.ActiveSince != "Fri 2026-05-15 10:00:00 UTC" {
		t.Fatalf("service resource fields = %+v", item)
	}
	if item.UnitFileState != "enabled" || item.Result != "success" || item.NRestarts != "2" || item.TasksCurrent != "6" || item.User != "app" || item.Restart != "always" || item.RestartSec != "5000000" || item.ExecStop != "/bin/stop" || item.ExecReload != "/bin/reload" {
		t.Fatalf("service extended fields = %+v", item)
	}
}

func TestParseServiceDetailsIgnoresInvalidMemoryCurrent(t *testing.T) {
	out := "__SSHM_SERVICE__\tapi.service\tloaded\tactive\trunning\tAPI Service\t/etc/systemd/system/api.service\t/data/api\t/data/api/server\t0\t456\t18446744073709551615"
	services, errText := parseServiceDetails(out)
	if errText != "" {
		t.Fatalf("errText = %q", errText)
	}
	if len(services) != 1 {
		t.Fatalf("services = %#v, want 1", services)
	}
	if services[0].MainPID != "" || services[0].ExecMainPID != "456" || services[0].MemoryCurrent != 0 {
		t.Fatalf("invalid memory should be hidden: %+v", services[0])
	}
}

func TestParseServiceExtraDetailRawSystemctlShow(t *testing.T) {
	out := strings.Join([]string{
		"Id=postfix.service",
		"LoadState=loaded",
		"ActiveState=active",
		"SubState=running",
		"Description=Postfix Mail Transport Agent",
		"FragmentPath=/usr/lib/systemd/system/postfix.service",
		"ExecStart={ path=/usr/sbin/postfix ; argv[]=/usr/sbin/postfix start ; status=0 }",
		"MainPID=3137",
		"ExecMainPID=0",
		"MemoryCurrent=7969177",
		"ActiveEnterTimestamp=Tue 2026-03-17 11:01:19 CST",
		"UnitFileState=enabled",
		"Result=success",
		"ExecMainStatus=0",
		"NRestarts=0",
		"TasksCurrent=4",
		"ControlGroup=/system.slice/postfix.service",
		"Slice=system.slice",
		"Restart=no",
		"RestartUSec=100ms",
	}, "\n")
	item, errText := parseServiceExtraDetail(out)
	if errText != "" {
		t.Fatalf("errText = %q", errText)
	}
	if item.Unit != "postfix.service" || item.UnitFileState != "enabled" || item.Result != "success" || item.ExecMainStatus != "0" || item.NRestarts != "0" {
		t.Fatalf("basic extra fields = %+v", item)
	}
	if item.TasksCurrent != "4" || item.ControlGroup != "/system.slice/postfix.service" || item.Slice != "system.slice" || item.Restart != "no" || item.RestartSec != "100ms" {
		t.Fatalf("extended fields = %+v", item)
	}
	if got := serviceProgramPath(item); got != "/usr/sbin/postfix" {
		t.Fatalf("program path = %q, want /usr/sbin/postfix", got)
	}
}

func TestServiceDetailRowsShowStatusAndDescription(t *testing.T) {
	m := Model{width: 120}
	rows := serviceDetailItemRows(m, serviceDetail{
		Unit:        "redis.service",
		Load:        "loaded",
		Active:      "failed",
		Sub:         "failed",
		Description: "Redis server",
	}, 14, 1)
	got := strings.Join(rows, "\n")
	if !strings.Contains(got, "Failed") || !strings.Contains(got, "State failed/failed") || !strings.Contains(got, "Desc Redis server") {
		t.Fatalf("service rows missing status or description:\n%s", got)
	}
}

func TestAddFormDoesNotShowHealthPorts(t *testing.T) {
	fields := addForm{
		Category: "prod",
		Name:     "api-01",
		HostName: "10.0.0.1",
		User:     "root",
		Port:     "22",
	}.fields()
	for _, field := range fields {
		if field.Label == "健康端口" {
			t.Fatalf("add form should not include health ports field: %+v", fields)
		}
	}
}

func TestServerDetailDoesNotShowHealthPorts(t *testing.T) {
	m := Model{
		width:              120,
		height:             40,
		selected:           0,
		detailSectionIndex: 1,
		states: []hostState{{
			Host: host.Host{Category: "prod", Name: "api-01"},
			Metrics: monitor.Metrics{
				Online: true,
				HealthPorts: []monitor.HealthPort{
					{Port: 80, Healthy: true},
				},
			},
		}},
	}
	view := ansi.Strip(m.renderDetail())
	for _, notWant := range []string{"健康端口", "Health ports"} {
		if strings.Contains(view, notWant) {
			t.Fatalf("server detail should not render %q:\n%s", notWant, view)
		}
	}
}

func TestServerDetailServicesAndContainersAreSummariesOnly(t *testing.T) {
	base := Model{
		appConfig:          config.AppConfig{Language: "zh"},
		width:              120,
		height:             60,
		selected:           0,
		detailSectionIndex: 1,
		states: []hostState{{
			Host: host.Host{Category: "prod", Name: "api-01"},
			Metrics: monitor.Metrics{
				Online:           true,
				ServiceAvailable: true,
				ServiceTotal:     2,
				ServiceRunning:   1,
				DockerAvailable:  true,
				FailedServices:   1,
				DockerTotal:      2,
				DockerRunning:    1,
				DockerStopped:    1,
			},
			ServiceDetails: []serviceDetail{
				{Unit: "api.service", Load: "loaded", Active: "failed", Sub: "failed", Description: "API service"},
				{Unit: "worker.service", Load: "loaded", Active: "active", Sub: "running", Description: "Worker service"},
			},
			ContainerDetails: []containerDetail{
				{Name: "api", Status: "Exited (1)", Image: "app:latest"},
				{Name: "redis", Status: "Up 1 hour", Image: "redis:7"},
			},
			LoginLoading: true,
		}},
	}
	m := base
	m.detailSectionIndex = 1
	view := ansi.Strip(m.renderDetail())
	for _, section := range m.detailSectionNames() {
		if section == "服务状态" || section == "容器" || section == "Services" || section == "Containers" {
			t.Fatalf("detail should not keep service/container as top-level sections: %+v", m.detailSectionNames())
		}
	}
	for _, notWant := range []string{"服务详情", "Service details", "Problems", "异常", "api.service", "worker.service", "redis:7"} {
		if strings.Contains(view, notWant) {
			t.Fatalf("resource detail should render service/container previews only, found %q:\n%s", notWant, view)
		}
	}
	for _, want := range []string{"服务", "容器", "总数", "运行", "停止", "故障"} {
		if !strings.Contains(view, want) {
			t.Fatalf("resource detail should include preview label %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "加载中") {
		t.Fatalf("resource preview should not depend on login loading:\n%s", view)
	}
}

func TestServerDetailContainerPreviewShowsDockerUnavailable(t *testing.T) {
	m := Model{
		appConfig:          config.AppConfig{Language: "zh"},
		width:              120,
		height:             60,
		selected:           0,
		detailSectionIndex: 1,
		states: []hostState{{
			Host: host.Host{Category: "prod", Name: "api-01"},
			Metrics: monitor.Metrics{
				Online: true,
			},
		}},
	}
	view := ansi.Strip(m.renderDetail())
	if !strings.Contains(view, "未安装Docker") {
		t.Fatalf("container preview should show docker unavailable:\n%s", view)
	}
	containerIndex := strings.Index(view, "· 容器")
	if containerIndex < 0 {
		t.Fatalf("container preview missing:\n%s", view)
	}
	containerView := view[containerIndex:]
	for _, notWant := range []string{"总数", "运行", "停止", "故障"} {
		if strings.Contains(containerView, notWant) {
			t.Fatalf("unavailable container preview should not show count row %q:\n%s", notWant, view)
		}
	}
}

func TestServerDetailScrollUsesRenderedLineCount(t *testing.T) {
	m := Model{
		appConfig:          config.AppConfig{Language: "zh"},
		width:              120,
		height:             24,
		selected:           0,
		detailSectionIndex: 1,
		states: []hostState{{
			Host: host.Host{Category: "prod", Name: "api-01"},
			Metrics: monitor.Metrics{
				Online:           true,
				ServiceAvailable: true,
				DockerAvailable:  true,
				Disks: []monitor.DiskMetric{
					{Filesystem: "/dev/mapper/cs-root", Type: "xfs", Mountpoint: "/", Used: 109, Total: 375, Available: 266, AvailKnown: true},
					{Filesystem: "/dev/sda1", Type: "xfs", Mountpoint: "/boot", Used: 252, Total: 960, Available: 708, AvailKnown: true},
					{Filesystem: "/dev/sdb1", Type: "xfs", Mountpoint: "/mnt/ssdc1t", Used: 80, Total: 999, Available: 919, AvailKnown: true},
				},
			},
		}},
	}
	if maxScroll := m.detailMaxScroll(); maxScroll <= 0 {
		t.Fatalf("detailMaxScroll = %d, want positive rendered-line scroll", maxScroll)
	}
	m.detailScroll = m.detailMaxScroll()
	view := ansi.Strip(m.renderDetail())
	if !strings.Contains(view, "服务器详情") {
		t.Fatalf("detail header should remain visible while scrolled:\n%s", view)
	}
	if !strings.Contains(view, "Basic") && !strings.Contains(view, "基础") {
		t.Fatalf("detail tabs should remain visible while scrolled:\n%s", view)
	}
	if !strings.Contains(view, "返回 q/Esc") {
		t.Fatalf("detail help should remain visible while scrolled:\n%s", view)
	}
}

func TestServerDetailResourcePreviewUsesMonitorCountsWithoutDetailCollection(t *testing.T) {
	m := Model{
		appConfig:          config.AppConfig{Language: "zh"},
		width:              120,
		height:             60,
		selected:           0,
		detailSectionIndex: 1,
		states: []hostState{{
			Host: host.Host{Category: "prod", Name: "api-01"},
			Metrics: monitor.Metrics{
				Online:           true,
				ServiceAvailable: true,
				ServiceTotal:     8,
				ServiceRunning:   6,
				ServiceStopped:   1,
				FailedServices:   1,
				DockerAvailable:  true,
				DockerTotal:      3,
				DockerRunning:    2,
				DockerStopped:    1,
			},
		}},
	}
	view := ansi.Strip(m.renderDetail())
	if strings.Contains(view, "加载中") {
		t.Fatalf("resource preview should not wait for detail collection:\n%s", view)
	}
	for _, want := range []string{"总数      8", "运行      6", "停止      0", "故障      1", "总数      3", "运行      2"} {
		if !strings.Contains(view, want) {
			t.Fatalf("resource preview should show monitor count %q:\n%s", want, view)
		}
	}
}

func TestServerDetailServicePreviewAvoidsMisleadingSingleFailedTotal(t *testing.T) {
	m := Model{
		appConfig:          config.AppConfig{Language: "zh"},
		width:              120,
		height:             60,
		selected:           0,
		detailSectionIndex: 1,
		states: []hostState{{
			Host: host.Host{Category: "prod", Name: "api-01"},
			Metrics: monitor.Metrics{
				Online:          true,
				DockerAvailable: true,
				FailedServices:  1,
			},
		}},
	}
	view := ansi.Strip(m.renderDetail())
	serviceIndex := strings.Index(view, "· 服务")
	containerIndex := strings.Index(view, "· 容器")
	if serviceIndex < 0 || containerIndex <= serviceIndex {
		t.Fatalf("service/container preview missing:\n%s", view)
	}
	serviceView := view[serviceIndex:containerIndex]
	if strings.Contains(serviceView, "总数") || strings.Contains(serviceView, "运行") || strings.Contains(serviceView, "停止") {
		t.Fatalf("single failed service preview should not pretend to know total/running/stopped:\n%s", view)
	}
	if !strings.Contains(serviceView, "故障") || !strings.Contains(serviceView, "1") {
		t.Fatalf("single failed service preview should show failed count:\n%s", view)
	}
}

func TestServerDetailProblemsDoNotDuplicateCollectionErrors(t *testing.T) {
	m := Model{
		appConfig:          config.AppConfig{Language: "zh"},
		width:              120,
		height:             40,
		selected:           0,
		detailSectionIndex: 1,
		states: []hostState{{
			Host:         host.Host{Category: "prod", Name: "api-01"},
			ServiceError: "signal: killed",
		}},
	}
	view := ansi.Strip(m.renderDetail())
	if strings.Contains(view, "signal: killed") {
		t.Fatalf("service detail should hide raw collection errors:\n%s", view)
	}
	if strings.Contains(view, "采集失败") {
		t.Fatalf("resource preview should not show collection failure text:\n%s", view)
	}
	if strings.Contains(view, "无法列出异常项") {
		t.Fatalf("service detail should not render a redundant problem section on collection failure:\n%s", view)
	}
}

func TestParseContainerDetailsWithDockerStats(t *testing.T) {
	out := strings.Join([]string{
		"__SSHM_CONTAINER__\tapi\tapp:latest\tUp 2 minutes\t80->80/tcp",
		"__SSHM_CONTAINER_STATS__\tapi\t0.12%\t32.4MiB / 1.9GiB\t1.65%",
		"__SSHM_CONTAINER_LIMIT__\t/api\t1500000000\t0\t0\t",
	}, "\n")
	items, errText := parseContainerDetails(out)
	if errText != "" {
		t.Fatalf("errText = %q", errText)
	}
	if len(items) != 1 {
		t.Fatalf("items = %#v, want 1", items)
	}
	if items[0].CPU != "0.12%" || items[0].Memory != "32.4M/1.9G" || items[0].MemPerc != "1.65%" {
		t.Fatalf("container stats = %+v", items[0])
	}
	if !items[0].CPULimitKnown || items[0].NanoCpus != 1500000000 {
		t.Fatalf("container cpu limit = %+v", items[0])
	}
}

func TestParseContainerExtraDetail(t *testing.T) {
	inspect := `{"Id":"abcdef1234567890","Created":"2026-05-15T10:00:00Z","Path":"/app","Args":["serve"],"Driver":"overlay2","Platform":"linux","SizeRw":1234,"SizeRootFs":5678,"State":{"Status":"running","StartedAt":"2026-05-15T10:01:00Z","FinishedAt":"0001-01-01T00:00:00Z","ExitCode":0,"Health":{"Status":"healthy"}},"HostConfig":{"RestartPolicy":{"Name":"unless-stopped"},"NanoCpus":2000000000,"CpuQuota":0,"CpuPeriod":0,"CpusetCpus":""},"Mounts":[{"Type":"bind","Source":"/data/app","Destination":"/app/data","RW":true}],"NetworkSettings":{"Networks":{"customer_default":{"IPAddress":"172.18.0.3","Gateway":"172.18.0.1","MacAddress":"02:42:ac:12:00:03","NetworkID":"networkabcdef123456","EndpointID":"endpointabcdef123456","Aliases":["api","customer-app-1"]}}}}`
	out := strings.Join([]string{
		"__SSHM_CONTAINER_INSPECT__\t" + inspect,
		"__SSHM_CONTAINER_SIZE__\t12.3MB (virtual 1.2GB)",
		"__SSHM_CONTAINER_BLOCKIO__\t1.2MB / 3.4MB",
	}, "\n")
	detail, errText := parseContainerExtraDetail(out)
	if errText != "" {
		t.Fatalf("errText = %q", errText)
	}
	if detail.ID != "abcdef1234567890" || detail.Size != "12.3MB" || detail.VirtualSize != "1.2GB" || detail.BlockIO != "1.2MB/3.4MB" {
		t.Fatalf("detail = %+v", detail)
	}
	if detail.SizeRW != 1234 || detail.SizeRootFS != 5678 || detail.HealthStatus != "healthy" || detail.RestartPolicy != "unless-stopped" {
		t.Fatalf("inspect fields = %+v", detail)
	}
	if detail.NanoCpus != 2000000000 {
		t.Fatalf("cpu limit fields = %+v", detail)
	}
	if len(detail.Mounts) != 1 || detail.Mounts[0].Source != "/data/app" || !detail.Mounts[0].RW {
		t.Fatalf("mounts = %+v", detail.Mounts)
	}
	if len(detail.Networks) != 1 || detail.Networks[0].Name != "customer_default" || detail.Networks[0].IPAddress != "172.18.0.3" || len(detail.Networks[0].Aliases) != 2 {
		t.Fatalf("networks = %+v", detail.Networks)
	}
}

func TestContainerCPULimitText(t *testing.T) {
	m := Model{resourceContainerExtra: containerExtraDetail{NanoCpus: 1500000000}}
	if got := m.containerCPULimitText(); got != "1.5 cores limit" {
		t.Fatalf("nano cpu limit = %q", got)
	}
	m = Model{resourceContainerExtra: containerExtraDetail{CpusetCpus: "0,1"}}
	if got := m.containerCPULimitText(); got != "CPU 0,1" {
		t.Fatalf("cpuset cpu limit = %q", got)
	}
	m = Model{}
	if got := m.containerCPULimitText(); got != "Unlimited" {
		t.Fatalf("unlimited cpu = %q", got)
	}
}

func TestContainerCardShowsCPULimitFromListData(t *testing.T) {
	m := Model{
		appConfig:         config.AppConfig{Language: "zh"},
		resourceHostIndex: 0,
		states: []hostState{{
			ContainerDetails: []containerDetail{{
				Name:          "api",
				Image:         "app:latest",
				Status:        "Up 2 minutes",
				CPU:           "81%",
				Memory:        "128M/1G",
				MemPerc:       "12%",
				CPULimitKnown: true,
			}},
		}},
	}
	card := ansi.Strip(m.resourceCard(resourceRef{Kind: resourceContainers, Index: 0}, false, 44))
	if !strings.Contains(card, "81%  未限制") {
		t.Fatalf("card should show cpu limit from list data:\n%s", card)
	}
}

func TestResourceCardMetaExtractsContainerAndServiceAge(t *testing.T) {
	m := Model{}
	if got := m.containerCardMeta(containerDetail{Status: "Up 4 weeks (healthy)"}); got != "28d" {
		t.Fatalf("container meta = %q, want 28d", got)
	}
	m.appConfig.Language = "zh"
	if got := m.containerCardMeta(containerDetail{Status: "Up 4 weeks (healthy)"}); got != "28天" {
		t.Fatalf("container zh meta = %q, want 28天", got)
	}
	if got := shortSystemdTimestampAge("bad timestamp", true); got != "" {
		t.Fatalf("bad systemd timestamp age = %q, want empty", got)
	}
}

func TestResourceFiltersSeparateContainersAndServices(t *testing.T) {
	m := Model{
		width:             120,
		resourceHostIndex: 0,
		resourceKind:      resourceContainers,
		resourceFilter:    resourceFilterProblems,
		states: []hostState{{
			ContainerDetails: []containerDetail{
				{Name: "api", Status: "Up 2 minutes", Managed: true},
				{Name: "worker", Status: "Restarting (1) 10 seconds ago", Managed: true},
			},
			ServiceDetails: []serviceDetail{
				{Unit: "nginx.service", Load: "loaded", Active: "active", Sub: "running", Managed: true},
				{Unit: "redis.service", Load: "loaded", Active: "failed", Sub: "failed", Managed: true},
			},
		}},
	}
	containerIndexes := m.filteredResourceIndexes()
	if len(containerIndexes) != 1 || containerIndexes[0] != (resourceRef{Kind: resourceContainers, Index: 1}) {
		t.Fatalf("container problem indexes = %#v, want worker only", containerIndexes)
	}
	m.resourceKind = resourceServices
	serviceIndexes := m.filteredResourceIndexes()
	if len(serviceIndexes) != 1 || serviceIndexes[0] != (resourceRef{Kind: resourceServices, Index: 1}) {
		t.Fatalf("service problem indexes = %#v, want redis only", serviceIndexes)
	}
}

func TestResourceAllIncludesContainersAndServices(t *testing.T) {
	m := Model{
		width:             120,
		resourceHostIndex: 0,
		resourceKind:      resourceAll,
		resourceFilter:    resourceFilterAll,
		states: []hostState{{
			ContainerDetails: []containerDetail{{Name: "api", Status: "Up 2 minutes", Managed: true}},
			ServiceDetails:   []serviceDetail{{Unit: "nginx.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/etc/systemd/system/nginx.service", ExecStart: "/usr/sbin/nginx", Managed: true}},
			PortDetails:      []portDetail{{Protocol: "tcp", Port: "22", Process: "sshd", PID: "123", Managed: true}},
		}},
	}
	indexes := m.filteredResourceIndexes()
	want := []resourceRef{{Kind: resourceContainers, Index: 0}, {Kind: resourceServices, Index: 0}, {Kind: resourcePorts, Index: 0}}
	if !reflect.DeepEqual(indexes, want) {
		t.Fatalf("resource all indexes = %#v, want %#v", indexes, want)
	}
}

func TestResourceServicesHideNotFoundInactiveDeadUnits(t *testing.T) {
	m := Model{
		width:             120,
		resourceHostIndex: 0,
		resourceKind:      resourceServices,
		resourceFilter:    resourceFilterAll,
		states: []hostState{{
			ServiceDetails: []serviceDetail{
				{Unit: "display-manager.service", Load: "not-found", Active: "inactive", Sub: "dead"},
				{Unit: "api.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/etc/systemd/system/api.service", ExecStart: "/data/api/server", Managed: true},
				{Unit: "worker.service", Load: "loaded", Active: "failed", Sub: "failed", Managed: true},
			},
		}},
	}
	indexes := m.filteredResourceIndexes()
	want := []resourceRef{{Kind: resourceServices, Index: 1}, {Kind: resourceServices, Index: 2}}
	if !reflect.DeepEqual(indexes, want) {
		t.Fatalf("resource service indexes = %#v, want %#v", indexes, want)
	}
}

func TestResourceServicesDiscoveryShowsUserManagedSignals(t *testing.T) {
	m := Model{
		width:             120,
		resourceHostIndex: 0,
		resourceAddKind:   resourceServices,
		resourceFilter:    resourceFilterAll,
		states: []hostState{{
			ServiceDetails: []serviceDetail{
				{Unit: "sshd.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/usr/lib/systemd/system/sshd.service", ExecStart: "/usr/sbin/sshd -D"},
				{Unit: "api.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/etc/systemd/system/api.service", ExecStart: "/data/api/server"},
				{Unit: "worker.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/usr/lib/systemd/system/worker.service", WorkingDirectory: "/opt/worker"},
				{Unit: "broken.service", Load: "loaded", Active: "failed", Sub: "failed", FragmentPath: "/usr/lib/systemd/system/broken.service"},
			},
		}},
	}
	indexes := m.resourceManageDiscoveredRefs()
	want := []resourceRef{
		{Kind: resourceServices, Index: 0},
		{Kind: resourceServices, Index: 1},
		{Kind: resourceServices, Index: 2},
		{Kind: resourceServices, Index: 3},
	}
	if !reflect.DeepEqual(indexes, want) {
		t.Fatalf("discovered service indexes = %#v, want %#v", indexes, want)
	}
}

func TestResourceManagerServiceLineShowsLocalizedStatusAndRawState(t *testing.T) {
	m := Model{
		appConfig:         config.AppConfig{Language: "zh"},
		resourceHostIndex: 0,
		states: []hostState{{
			ServiceDetails: []serviceDetail{{Unit: "api.service", Load: "loaded", Active: "active", Sub: "running"}},
		}},
	}
	line := ansi.Strip(m.resourceManageRefLine(resourceRef{Kind: resourceServices, Index: 0}, true, 80))
	if !strings.Contains(line, "运行") || !strings.Contains(line, "api.service") || !strings.Contains(line, "active/running") {
		t.Fatalf("service manager line should include localized status, name and raw state:\n%s", line)
	}
}

func TestResourceManagerStatusColumnAlignsNames(t *testing.T) {
	m := Model{
		appConfig:         config.AppConfig{Language: "zh"},
		resourceHostIndex: 0,
		states: []hostState{{
			ServiceDetails: []serviceDetail{
				{Unit: "api.service", Load: "loaded", Active: "active", Sub: "running"},
				{Unit: "broken-long-name.service", Load: "loaded", Active: "failed", Sub: "failed"},
			},
		}},
	}
	line1 := ansi.Strip(m.resourceManageRefLine(resourceRef{Kind: resourceServices, Index: 0}, false, 80))
	line2 := ansi.Strip(m.resourceManageRefLine(resourceRef{Kind: resourceServices, Index: 1}, false, 80))
	if strings.Index(line1, "api.service") != strings.Index(line2, "broken-long-name.service") {
		t.Fatalf("resource manager names should align:\n%s\n%s", line1, line2)
	}
	if strings.Index(line1, "active/running") != strings.Index(line2, "failed/failed") {
		t.Fatalf("resource manager raw state column should align:\n%s\n%s", line1, line2)
	}
}

func TestResourceManagerAddedLineMatchesDiscoveredLine(t *testing.T) {
	m := Model{
		appConfig:         config.AppConfig{Language: "zh"},
		resourceHostIndex: 0,
		states: []hostState{{
			ServiceDetails: []serviceDetail{{Unit: "api.service", Load: "loaded", Active: "failed", Sub: "failed"}},
		}},
	}
	item := config.ManagedResource{
		Server:         "prod/api-01",
		Kind:           config.ResourceKindService,
		Name:           "api.service",
		StartCommand:   "systemctl start api.service",
		RestartCommand: "systemctl restart api.service",
	}
	left := ansi.Strip(m.resourceManageRefLine(resourceRef{Kind: resourceServices, Index: 0}, false, 80))
	right := ansi.Strip(m.resourceManageFavoriteLine(item, false, 80))
	if left != right {
		t.Fatalf("added line should match discovered line:\nleft:  %s\nright: %s", left, right)
	}
	if strings.Contains(right, "命令") || strings.Contains(right, "commands") {
		t.Fatalf("added line should not show command marker:\n%s", right)
	}
}

func TestResourceServicesUseGenericDiscoveryRules(t *testing.T) {
	m := Model{
		width:             120,
		resourceHostIndex: 0,
		resourceAddKind:   resourceServices,
		resourceFilter:    resourceFilterAll,
		states: []hostState{{
			ServiceDetails: []serviceDetail{
				{Unit: "cron.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/usr/lib/systemd/system/cron.service", ExecStart: "/usr/sbin/cron -f"},
				{Unit: "boot-helper.service", Load: "loaded", Active: "active", Sub: "exited", FragmentPath: "/usr/lib/systemd/system/boot-helper.service", ExecStart: "/usr/lib/systemd/helper"},
				{Unit: "inactive-custom.service", Load: "loaded", Active: "inactive", Sub: "dead", FragmentPath: "/usr/lib/systemd/system/inactive-custom.service"},
				{Unit: "x-ui.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/etc/systemd/system/x-ui.service", ExecStart: "/usr/local/x-ui/x-ui"},
				{Unit: "openvpn-server@server.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/usr/lib/systemd/system/openvpn-server@.service", ExecStart: "/usr/sbin/openvpn --config /etc/openvpn/server.conf"},
				{Unit: "policy-routes@ens5.service", Load: "loaded", Active: "active", Sub: "exited", FragmentPath: "/usr/lib/systemd/system/policy-routes@.service", ExecStart: "/usr/sbin/ip route"},
				{Unit: "chronyd.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/usr/lib/systemd/system/chronyd.service", ExecStart: "/usr/sbin/chronyd"},
				{Unit: "docker.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/usr/lib/systemd/system/docker.service", ExecStart: "/usr/bin/dockerd -H fd://"},
				{Unit: "containerd.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/usr/lib/systemd/system/containerd.service", ExecStart: "/usr/bin/containerd"},
			},
		}},
	}
	indexes := m.resourceManageDiscoveredRefs()
	want := []resourceRef{
		{Kind: resourceServices, Index: 0},
		{Kind: resourceServices, Index: 1},
		{Kind: resourceServices, Index: 2},
		{Kind: resourceServices, Index: 3},
		{Kind: resourceServices, Index: 4},
		{Kind: resourceServices, Index: 5},
		{Kind: resourceServices, Index: 6},
		{Kind: resourceServices, Index: 7},
		{Kind: resourceServices, Index: 8},
	}
	if !reflect.DeepEqual(indexes, want) {
		t.Fatalf("discovered service indexes = %#v, want %#v", indexes, want)
	}
}

func TestResourceServicesHideSystemHelpersFromRealServerSet(t *testing.T) {
	m := Model{
		width:             120,
		resourceHostIndex: 0,
		resourceAddKind:   resourceServices,
		resourceFilter:    resourceFilterAll,
		states: []hostState{{
			ServiceDetails: []serviceDetail{
				{Unit: "chronyd.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/usr/lib/systemd/system/chronyd.service", ExecStart: "/usr/sbin/chronyd"},
				{Unit: "containerd.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/usr/lib/systemd/system/containerd.service", ExecStart: "/usr/bin/containerd"},
				{Unit: "docker.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/usr/lib/systemd/system/docker.service", ExecStart: "/usr/bin/dockerd"},
				{Unit: "ipsec.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/usr/lib/systemd/system/ipsec.service", ExecStart: "/usr/libexec/ipsec/starter"},
				{Unit: "network.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/run/systemd/generator.late/network.service", ExecStart: "/etc/rc.d/init.d/network start"},
				{Unit: "openvpn-server@server.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/usr/lib/systemd/system/openvpn-server@.service", ExecStart: "/usr/sbin/openvpn --config /etc/openvpn/server.conf"},
				{Unit: "postfix.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/usr/lib/systemd/system/postfix.service", ExecStart: "/usr/sbin/postfix start"},
				{Unit: "rpcbind.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/usr/lib/systemd/system/rpcbind.service", ExecStart: "/usr/bin/rpcbind -w -f"},
				{Unit: "x-ui.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/etc/systemd/system/x-ui.service", ExecStart: "/usr/local/x-ui/x-ui"},
				{Unit: "xl2tpd.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/usr/lib/systemd/system/xl2tpd.service", ExecStart: "/usr/sbin/xl2tpd -D"},
				{Unit: "openvpn-iptables.service", Load: "loaded", Active: "active", Sub: "exited", FragmentPath: "/etc/systemd/system/openvpn-iptables.service", ExecStart: "/usr/sbin/iptables -t nat -A POSTROUTING"},
			},
		}},
	}
	indexes := m.resourceManageDiscoveredRefs()
	want := []resourceRef{
		{Kind: resourceServices, Index: 0},
		{Kind: resourceServices, Index: 1},
		{Kind: resourceServices, Index: 2},
		{Kind: resourceServices, Index: 3},
		{Kind: resourceServices, Index: 4},
		{Kind: resourceServices, Index: 5},
		{Kind: resourceServices, Index: 6},
		{Kind: resourceServices, Index: 7},
		{Kind: resourceServices, Index: 8},
		{Kind: resourceServices, Index: 9},
		{Kind: resourceServices, Index: 10},
	}
	if !reflect.DeepEqual(indexes, want) {
		t.Fatalf("real service indexes = %#v, want %#v", indexes, want)
	}
}

func TestResourceServicesShowPackageServiceOwningPort(t *testing.T) {
	m := Model{
		width:             120,
		resourceHostIndex: 0,
		resourceAddKind:   resourceServices,
		resourceFilter:    resourceFilterAll,
		states: []hostState{{
			ServiceDetails: []serviceDetail{
				{Unit: "nginx.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/usr/lib/systemd/system/nginx.service", ExecStart: "/usr/sbin/nginx -g 'daemon off;'", MainPID: "100"},
				{Unit: "sshd.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/usr/lib/systemd/system/sshd.service", ExecStart: "/usr/sbin/sshd -D", MainPID: "200"},
				{Unit: "x-ui.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/usr/lib/systemd/system/x-ui.service", ExecStart: "/usr/local/x-ui/x-ui", MainPID: "300"},
				{Unit: "chronyd.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/usr/lib/systemd/system/chronyd.service", ExecStart: "/usr/sbin/chronyd", MainPID: "400"},
			},
			PortDetails: []portDetail{
				{Protocol: "tcp", Port: "80", LocalAddress: "0.0.0.0:80", Process: "nginx", PID: "100", Count: 1},
				{Protocol: "tcp", Port: "22", LocalAddress: "0.0.0.0:22", Process: "sshd", PID: "200", Count: 1},
				{Protocol: "tcp", Port: "2053", LocalAddress: "*:2053", Process: "x-ui", PID: "301", Count: 1},
				{Protocol: "udp", Port: "323", LocalAddress: "127.0.0.1:323", Process: "chronyd", PID: "400", Count: 1},
			},
		}},
	}
	indexes := m.resourceManageDiscoveredRefs()
	want := []resourceRef{
		{Kind: resourceServices, Index: 0},
		{Kind: resourceServices, Index: 1},
		{Kind: resourceServices, Index: 2},
		{Kind: resourceServices, Index: 3},
	}
	if !reflect.DeepEqual(indexes, want) {
		t.Fatalf("service indexes = %#v, want all discovered services", indexes)
	}
}

func TestResourceProcessesShowStandaloneListenersOnly(t *testing.T) {
	m := Model{
		width:             120,
		resourceHostIndex: 0,
		resourceAddKind:   resourceProcesses,
		resourceFilter:    resourceFilterAll,
		states: []hostState{{
			ServiceDetails: []serviceDetail{
				{Unit: "nginx.service", Load: "loaded", Active: "active", Sub: "running", ExecStart: "/usr/sbin/nginx", MainPID: "100"},
			},
			PortDetails: []portDetail{
				{Protocol: "tcp", Port: "80", LocalAddress: "0.0.0.0:80", Process: "nginx", PID: "100", Count: 1},
				{Protocol: "tcp", Port: "8080", LocalAddress: "0.0.0.0:8080", Process: "go-api", PID: "200", Count: 1},
				{Protocol: "tcp", Port: "1080", LocalAddress: "0.0.0.0:1080", Process: "docker-proxy", PID: "300", Container: "socks5", Count: 1},
				{Protocol: "tcp", Port: "22", LocalAddress: "0.0.0.0:22", Process: "sshd", PID: "400", Count: 1},
			},
		}},
	}
	indexes := m.resourceManageDiscoveredRefs()
	want := []resourceRef{
		{Kind: resourceProcesses, Index: 0},
		{Kind: resourceProcesses, Index: 1},
		{Kind: resourceProcesses, Index: 2},
		{Kind: resourceProcesses, Index: 3},
	}
	if !reflect.DeepEqual(indexes, want) {
		t.Fatalf("process indexes = %#v, want all discovered processes", indexes)
	}
}

func TestResourceProcessesHideCgroupOwnedListeners(t *testing.T) {
	m := Model{
		width:             120,
		resourceHostIndex: 0,
		resourceAddKind:   resourceProcesses,
		resourceFilter:    resourceFilterAll,
		states: []hostState{{
			ServiceDetails: []serviceDetail{
				{Unit: "x-ui.service", Load: "loaded", Active: "active", Sub: "running"},
			},
			PortDetails: []portDetail{
				{Protocol: "tcp", Port: "11111", LocalAddress: "127.0.0.1:11111", Process: "xray-linux-amd64", PID: "200", ServiceUnit: "x-ui.service", Count: 1},
			},
		}},
	}
	indexes := m.resourceManageDiscoveredRefs()
	want := []resourceRef{{Kind: resourceProcesses, Index: 0}}
	if !reflect.DeepEqual(indexes, want) {
		t.Fatalf("process indexes = %#v, want all discovered processes", indexes)
	}
}

func TestManagedProcessResourceShowsInManagedScope(t *testing.T) {
	m := Model{
		width:             120,
		resourceHostIndex: 0,
		resourceKind:      resourceProcesses,
		resourceScope:     resourceScopeDiscovered,
		resourceFilter:    resourceFilterAll,
		resourceFile: config.ResourcesFile{Items: []config.ManagedResource{
			{Server: "prod/api-01", Kind: config.ResourceKindProcess, Name: "go-api"},
		}},
		states: []hostState{{
			Host:        host.Host{Category: "prod", Name: "api-01"},
			PortDetails: []portDetail{{Protocol: "tcp", Port: "8080", LocalAddress: "0.0.0.0:8080", Process: "go-api", PID: "200", Count: 1}},
		}},
	}
	m.applyManagedResources(0)
	indexes := m.filteredResourceIndexes()
	want := []resourceRef{{Kind: resourceProcesses, Index: 0}}
	if !reflect.DeepEqual(indexes, want) {
		t.Fatalf("managed process indexes = %#v, want %#v", indexes, want)
	}
	if !m.states[0].PortDetails[0].ProcessManaged {
		t.Fatalf("process port should be marked managed: %+v", m.states[0].PortDetails[0])
	}
}

func TestManagedMissingProcessStillShowsInResourceList(t *testing.T) {
	m := Model{
		width:             120,
		resourceHostIndex: 0,
		resourceKind:      resourceProcesses,
		resourceScope:     resourceScopeDiscovered,
		resourceFilter:    resourceFilterAll,
		resourceFile: config.ResourcesFile{Items: []config.ManagedResource{
			{Server: "prod/api-01", Kind: config.ResourceKindProcess, Name: "go-api"},
		}},
		states: []hostState{{
			Host: host.Host{Category: "prod", Name: "api-01"},
		}},
	}
	m.applyManagedResources(0)
	indexes := m.filteredResourceIndexes()
	want := []resourceRef{{Kind: resourceProcesses, Index: 0}}
	if !reflect.DeepEqual(indexes, want) {
		t.Fatalf("managed missing process indexes = %#v, want %#v", indexes, want)
	}
	item := m.states[0].PortDetails[0]
	if !item.ProcessManaged || !item.Missing || item.Process != "go-api" {
		t.Fatalf("managed missing process = %+v", item)
	}
}

func TestManagedResourceMissingStillShows(t *testing.T) {
	m := Model{
		width:             120,
		resourceHostIndex: 0,
		resourceKind:      resourceServices,
		resourceScope:     resourceScopeDiscovered,
		resourceFilter:    resourceFilterAll,
		resourceFile: config.ResourcesFile{Items: []config.ManagedResource{
			{Server: "prod/api-01", Kind: config.ResourceKindService, Name: "api.service"},
		}},
		states: []hostState{{Host: host.Host{Category: "prod", Name: "api-01"}}},
	}
	m.applyManagedResources(0)
	indexes := m.filteredResourceIndexes()
	if len(indexes) != 1 || indexes[0] != (resourceRef{Kind: resourceServices, Index: 0}) {
		t.Fatalf("managed indexes = %#v, want missing service", indexes)
	}
	item := m.states[0].ServiceDetails[0]
	if !item.Managed || !item.Missing || serviceDetailKind(item) != "missing" {
		t.Fatalf("managed service = %+v, kind %s", item, serviceDetailKind(item))
	}
}

func TestToggleManagedResourceSavesConfig(t *testing.T) {
	home := t.TempDir()
	m := Model{
		home:              home,
		width:             120,
		resourceHostIndex: 0,
		resourceKind:      resourceContainers,
		resourceScope:     resourceScopeDiscovered,
		resourceFilter:    resourceFilterAll,
		states: []hostState{{
			Host:             host.Host{Category: "prod", Name: "api-01"},
			ContainerDetails: []containerDetail{{Name: "api", Status: "Up 1 second", Managed: true}},
		}},
		resourceFile: config.ResourcesFile{Items: []config.ManagedResource{{
			Server: "prod/api-01",
			Kind:   config.ResourceKindContainer,
			Name:   "api",
		}}},
	}
	next, _ := m.toggleManagedResource()
	got := next.(Model)
	file, _, err := config.LoadResources(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Items) != 1 || file.Items[0].Name != "api" || file.Items[0].Kind != config.ResourceKindContainer || !file.Items[0].Favorite {
		t.Fatalf("resources file = %#v", file)
	}
	if file.Items[0].StartCommand != "" || file.Items[0].LogCommand != "" {
		t.Fatalf("docker favorite should not create managed commands: %#v", file.Items[0])
	}
	next, _ = got.toggleManagedResource()
	file, _, err = config.LoadResources(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Items) != 0 {
		t.Fatalf("resources file after unfavorite = %#v", file)
	}
}

func TestResourceManagerAddsAndRemovesFavorite(t *testing.T) {
	home := t.TempDir()
	m := Model{
		home:              home,
		width:             120,
		resourceHostIndex: 0,
		resourceKind:      resourceServices,
		resourceScope:     resourceScopeDiscovered,
		resourceFilter:    resourceFilterAll,
		states: []hostState{{
			Host:           host.Host{Category: "prod", Name: "api-01"},
			ServiceDetails: []serviceDetail{{Unit: "api.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/etc/systemd/system/api.service"}},
		}},
	}
	next, _ := m.startResourceAdd()
	got := next.(Model)
	if got.mode != modeResourceAdd || got.resourceAddKind != resourceServices {
		t.Fatalf("mode/kind = %v/%v, want resource manager services", got.mode, got.resourceAddKind)
	}
	next, _ = got.toggleResourceManagerFavorite()
	got = next.(Model)
	file, _, err := config.LoadResources(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Items) != 1 || file.Items[0].Name != "api.service" || file.Items[0].Kind != config.ResourceKindService {
		t.Fatalf("resources after add = %#v", file.Items)
	}
	if got.resourceManagePane != 0 {
		t.Fatalf("pane after add = %d, want discovered", got.resourceManagePane)
	}
	got.resourceManagePane = 1
	next, _ = got.updateResourceAdd(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	got = next.(Model)
	if got.mode != modeConfirmAction || got.confirm.Kind != confirmRemoveResource {
		t.Fatalf("mode/confirm after x = %v/%v, want remove confirmation", got.mode, got.confirm.Kind)
	}
	file, _, err = config.LoadResources(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Items) != 1 {
		t.Fatalf("resources removed before confirmation: %#v", file.Items)
	}
	next, _ = got.updateConfirmAction(tea.KeyMsg{Type: tea.KeyEnter})
	got = next.(Model)
	file, _, err = config.LoadResources(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Items) != 0 {
		t.Fatalf("resources after remove = %#v", file.Items)
	}
	if got.mode != modeResourceAdd {
		t.Fatalf("mode after remove = %v, want resource manager", got.mode)
	}
}

func TestResourceListXRemovesManagedResourceAfterConfirmation(t *testing.T) {
	home := t.TempDir()
	if err := config.SaveResources(home, config.ResourcesFile{Items: []config.ManagedResource{{
		Server: "prod/api-01",
		Kind:   config.ResourceKindService,
		Name:   "api.service",
	}}}); err != nil {
		t.Fatal(err)
	}
	file, _, err := config.LoadResources(home)
	if err != nil {
		t.Fatal(err)
	}
	m := Model{
		home:              home,
		width:             120,
		resourceHostIndex: 0,
		resourceKind:      resourceServices,
		resourceFilter:    resourceFilterAll,
		resourceFile:      file,
		states: []hostState{{
			Host:           host.Host{Category: "prod", Name: "api-01"},
			ServiceDetails: []serviceDetail{{Unit: "api.service", Load: "loaded", Active: "active", Sub: "running", Managed: true}},
		}},
	}
	next, _ := m.updateResourceList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	got := next.(Model)
	if got.mode != modeConfirmAction || got.confirm.Kind != confirmRemoveResource {
		t.Fatalf("mode/confirm after x = %v/%v, want remove confirmation", got.mode, got.confirm.Kind)
	}
	next, _ = got.updateConfirmAction(tea.KeyMsg{Type: tea.KeyEnter})
	got = next.(Model)
	if got.mode != modeResourceList {
		t.Fatalf("mode after confirm = %v, want resource list", got.mode)
	}
	file, _, err = config.LoadResources(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Items) != 0 {
		t.Fatalf("resources after x remove = %#v", file.Items)
	}
}

func TestResourceListXDoesNotRemoveDockerFavorite(t *testing.T) {
	home := t.TempDir()
	if err := config.SaveResources(home, config.ResourcesFile{Items: []config.ManagedResource{{
		Server:   "prod/api-01",
		Kind:     config.ResourceKindContainer,
		Name:     "api",
		Favorite: true,
	}}}); err != nil {
		t.Fatal(err)
	}
	file, _, err := config.LoadResources(home)
	if err != nil {
		t.Fatal(err)
	}
	m := Model{
		home:              home,
		width:             120,
		resourceHostIndex: 0,
		resourceKind:      resourceContainers,
		resourceFilter:    resourceFilterAll,
		resourceFile:      file,
		states: []hostState{{
			Host:             host.Host{Category: "prod", Name: "api-01"},
			ContainerDetails: []containerDetail{{Name: "api", Status: "Up 1 second", Managed: true, Favorite: true}},
		}},
	}
	next, _ := m.updateResourceList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	got := next.(Model)
	if got.mode == modeConfirmAction {
		t.Fatal("docker x should not open remove confirmation")
	}
	if !strings.Contains(got.status, "cannot be deleted") {
		t.Fatalf("status = %q, want cannot delete prompt", got.status)
	}
	file, _, err = config.LoadResources(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Items) != 1 || !file.Items[0].Favorite {
		t.Fatalf("docker favorite should remain after x: %#v", file.Items)
	}
}

func TestResourceActionShortcutsAreConsistentOnListAndDetail(t *testing.T) {
	base := Model{
		width:             120,
		resourceHostIndex: 0,
		resourceKind:      resourceServices,
		resourceFilter:    resourceFilterAll,
		states: []hostState{{
			Host:           host.Host{Category: "prod", Name: "api-01"},
			ServiceDetails: []serviceDetail{{Unit: "api.service", Load: "loaded", Active: "active", Sub: "running", Managed: true}},
		}},
	}

	for key, action := range map[rune]resourceActionKind{
		's': resourceActionStart,
		'p': resourceActionStop,
		'c': resourceActionRestart,
	} {
		next, _ := base.updateResourceList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})
		got := next.(Model)
		if got.mode != modeResourceConfirm || got.resourceAction != action {
			t.Fatalf("list key %q mode/action = %v/%v, want confirm/%v", key, got.mode, got.resourceAction, action)
		}

		detail := base
		detail.mode = modeResourceDetail
		next, _ = detail.updateResourceDetail(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})
		got = next.(Model)
		if got.mode != modeResourceConfirm || got.resourceAction != action {
			t.Fatalf("detail key %q mode/action = %v/%v, want confirm/%v", key, got.mode, got.resourceAction, action)
		}
	}
}

func TestResourceDetailRRefreshesInsteadOfRestarting(t *testing.T) {
	m := Model{
		width:             120,
		mode:              modeResourceDetail,
		resourceHostIndex: 0,
		resourceKind:      resourceServices,
		resourceFilter:    resourceFilterAll,
		states: []hostState{{
			Host:           host.Host{Category: "prod", Name: "api-01"},
			ServiceDetails: []serviceDetail{{Unit: "api.service", Load: "loaded", Active: "active", Sub: "running", Managed: true}},
		}},
	}
	next, _ := m.updateResourceDetail(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	got := next.(Model)
	if got.mode == modeResourceConfirm || got.resourceAction == resourceActionRestart {
		t.Fatalf("detail r should refresh, not restart: mode/action=%v/%v", got.mode, got.resourceAction)
	}
	if !got.resourceLoading || got.resourceLoadingKind != resourceServices {
		t.Fatalf("detail r should start resource refresh: loading=%v kind=%v", got.resourceLoading, got.resourceLoadingKind)
	}
}

func TestResourceListTPinsDiscoveredContainer(t *testing.T) {
	home := t.TempDir()
	m := Model{
		home:              home,
		width:             120,
		resourceHostIndex: 0,
		resourceKind:      resourceContainers,
		resourceFilter:    resourceFilterAll,
		states: []hostState{{
			Host:             host.Host{Category: "prod", Name: "api-01"},
			ContainerDetails: []containerDetail{{Name: "api", Status: "Up 1 second"}},
		}},
	}
	next, _ := m.updateResourceList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	got := next.(Model)
	if !strings.Contains(got.status, "Pinned") {
		t.Fatalf("status = %q, want pinned", got.status)
	}
	file, _, err := config.LoadResources(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Items) != 1 || !file.Items[0].Pinned || file.Items[0].PinnedOrder == 0 {
		t.Fatalf("pinned resource not saved: %#v", file.Items)
	}
}

func TestResourceSortKeepsPinnedFirstThenSortsByCPU(t *testing.T) {
	m := Model{
		width:             120,
		resourceHostIndex: 0,
		resourceKind:      resourceContainers,
		resourceFilter:    resourceFilterAll,
		resourceSort:      resourceSortCPU,
		resourceFile: config.ResourcesFile{Items: []config.ManagedResource{
			{Server: "prod/api-01", Kind: config.ResourceKindContainer, Name: "api", Pinned: true, PinnedOrder: 1},
		}},
		states: []hostState{{
			Host: host.Host{Category: "prod", Name: "api-01"},
			ContainerDetails: []containerDetail{
				{Name: "api", Status: "Up 1 second", CPU: "10%"},
				{Name: "db", Status: "Up 1 second", CPU: "90%"},
				{Name: "web", Status: "Up 1 second", CPU: "50%"},
			},
		}},
	}
	refs := m.filteredResourceIndexes()
	names := []string{}
	for _, ref := range refs {
		name, _ := m.resourceNameForRef(ref)
		names = append(names, name)
	}
	want := []string{"api", "db", "web"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("sorted names = %#v, want %#v", names, want)
	}
}

func TestResourceSortShortcutCyclesSortMode(t *testing.T) {
	m := Model{
		width:             120,
		resourceHostIndex: 0,
		resourceKind:      resourceContainers,
		resourceFilter:    resourceFilterAll,
		states: []hostState{{
			Host:             host.Host{Category: "prod", Name: "api-01"},
			ContainerDetails: []containerDetail{{Name: "api", Status: "Up 1 second"}},
		}},
	}
	next, _ := m.updateResourceList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	got := next.(Model)
	if got.resourceSort != resourceSortStatus || !strings.Contains(got.status, "Sort") {
		t.Fatalf("sort after y = %v status=%q, want status sort", got.resourceSort, got.status)
	}
}

func TestResourceManagerXDoesNotRemoveDockerFavorite(t *testing.T) {
	home := t.TempDir()
	file := config.ResourcesFile{Items: []config.ManagedResource{{
		Server:   "prod/api-01",
		Kind:     config.ResourceKindContainer,
		Name:     "api",
		Favorite: true,
	}}}
	if err := config.SaveResources(home, file); err != nil {
		t.Fatal(err)
	}
	m := Model{
		home:                        home,
		width:                       120,
		resourceHostIndex:           0,
		resourceAddKind:             resourceContainers,
		resourceManagePane:          1,
		resourceManageFavoriteIndex: 0,
		resourceFile:                file,
		states: []hostState{{
			Host:             host.Host{Category: "prod", Name: "api-01"},
			ContainerDetails: []containerDetail{{Name: "api", Status: "Up 1 second", Managed: true, Favorite: true}},
		}},
	}
	next, _ := m.updateResourceAdd(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	got := next.(Model)
	if got.mode == modeConfirmAction {
		t.Fatal("docker manager x should not open remove confirmation")
	}
	if !strings.Contains(got.status, "cannot be deleted") {
		t.Fatalf("status = %q, want cannot delete prompt", got.status)
	}
	saved, _, err := config.LoadResources(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Items) != 1 || !saved.Items[0].Favorite {
		t.Fatalf("docker favorite should remain after manager x: %#v", saved.Items)
	}
}

func TestResourceLogScrollUsesViewportWindow(t *testing.T) {
	m := Model{height: 8, resourceLogOutput: strings.Join([]string{"l1", "l2", "l3", "l4", "l5", "l6", "l7"}, "\n")}
	next, _ := m.updateResourceLog(tea.KeyMsg{Type: tea.KeyDown})
	got := next.(Model)
	if got.resourceLogScroll != 1 {
		t.Fatalf("scroll after down = %d, want 1", got.resourceLogScroll)
	}
	start, end := resourceLogWindowRange(len(got.resourceLogLines()), got.resourceLogScroll, got.resourceLogBodyHeight())
	if start != 1 || end != 5 {
		t.Fatalf("window after down = %d-%d, want 1-5", start, end)
	}
	got.resourceLogScroll = 10
	next, _ = got.updateResourceLog(tea.KeyMsg{Type: tea.KeyDown})
	got = next.(Model)
	if got.resourceLogScroll != 3 {
		t.Fatalf("scroll should clamp to max, got %d want 3", got.resourceLogScroll)
	}
}

func TestResourceLogHeaderShowsNavigationAndRange(t *testing.T) {
	m := Model{
		appConfig:       config.AppConfig{Language: "zh"},
		width:           120,
		height:          8,
		resourceLogKind: resourceServices,
		resourceLogName: "api.service",
		resourceLogOutput: strings.Join([]string{
			"l1", "l2", "l3", "l4", "l5", "l6",
		}, "\n"),
		states: []hostState{{Host: host.Host{Category: "prod", Name: "api-01"}}},
	}
	view := ansi.Strip(m.renderResourceLog())
	renderedLines := strings.Split(view, "\n")
	if len(renderedLines) > m.height {
		t.Fatalf("resource log view rendered %d lines, want <= %d:\n%s", len(renderedLines), m.height, view)
	}
	if !strings.Contains(renderedLines[0], "资源") || !strings.Contains(renderedLines[0], "日志") {
		t.Fatalf("resource log header should stay on first line:\n%s", view)
	}
	for _, want := range []string{"资源", "日志", "[prod] api-01", "服务", "api.service", "1-4/6", "滚动 ↑↓/jk"} {
		if !strings.Contains(view, want) {
			t.Fatalf("resource log view missing %q:\n%s", want, view)
		}
	}
}

func TestResourceLogLongLinesDoNotChangeRenderedHeight(t *testing.T) {
	longLine := strings.Repeat("2026-03-04T07:03:43.052Z WireGuard Config syncing ", 8)
	m := Model{
		appConfig:       config.AppConfig{Language: "zh"},
		width:           80,
		height:          8,
		resourceLogKind: resourceContainers,
		resourceLogName: "wireguard",
		resourceLogOutput: strings.Join([]string{
			longLine,
			"short",
			longLine,
			"short",
			longLine,
			"short",
		}, "\n"),
		states: []hostState{{Host: host.Host{Category: "aws", Name: "vpn"}}},
	}
	view := ansi.Strip(m.renderResourceLog())
	renderedLines := strings.Split(view, "\n")
	if len(renderedLines) > m.height {
		t.Fatalf("resource log with long lines rendered %d lines, want <= %d:\n%s", len(renderedLines), m.height, view)
	}
	if !strings.Contains(renderedLines[0], "资源") || !strings.Contains(renderedLines[0], "日志") {
		t.Fatalf("resource log header should stay visible:\n%s", view)
	}
	next, _ := m.updateResourceLog(tea.KeyMsg{Type: tea.KeyDown})
	scrolled := ansi.Strip(next.(Model).renderResourceLog())
	if got := len(strings.Split(scrolled, "\n")); got > m.height {
		t.Fatalf("scrolled resource log rendered %d lines, want <= %d:\n%s", got, m.height, scrolled)
	}
}

func TestResourceManagerTabSwitchesPaneAndGSwitchesType(t *testing.T) {
	m := Model{resourceAddKind: resourceServices, resourceManagePane: 0}
	next, _ := m.updateResourceAdd(tea.KeyMsg{Type: tea.KeyTab})
	got := next.(Model)
	if got.resourceAddKind != resourceServices || got.resourceManagePane != 1 {
		t.Fatalf("after tab kind/pane = %v/%d, want services/1", got.resourceAddKind, got.resourceManagePane)
	}
	next, _ = got.updateResourceAdd(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	got = next.(Model)
	if got.resourceAddKind != resourceProcesses || got.resourceManagePane != 1 {
		t.Fatalf("after g kind/pane = %v/%d, want processes/1", got.resourceAddKind, got.resourceManagePane)
	}
	next, _ = got.updateResourceAdd(tea.KeyMsg{Type: tea.KeyLeft})
	got = next.(Model)
	if got.resourceAddKind != resourceServices || got.resourceManagePane != 1 {
		t.Fatalf("after left kind/pane = %v/%d, want services/1", got.resourceAddKind, got.resourceManagePane)
	}
	next, _ = got.updateResourceAdd(tea.KeyMsg{Type: tea.KeyRight})
	got = next.(Model)
	if got.resourceAddKind != resourceProcesses || got.resourceManagePane != 1 {
		t.Fatalf("after right kind/pane = %v/%d, want processes/1", got.resourceAddKind, got.resourceManagePane)
	}
	next, _ = got.updateResourceAdd(tea.KeyMsg{Type: tea.KeyCtrlI})
	got = next.(Model)
	if got.resourceAddKind != resourceProcesses || got.resourceManagePane != 0 {
		t.Fatalf("after ctrl+i kind/pane = %v/%d, want processes/0", got.resourceAddKind, got.resourceManagePane)
	}
}

func TestResourceCommandEditSavesCustomCommand(t *testing.T) {
	home := t.TempDir()
	if err := config.SaveResources(home, config.ResourcesFile{Items: []config.ManagedResource{{
		Server:         "prod/api-01",
		Kind:           config.ResourceKindService,
		Name:           "api.service",
		RestartCommand: "systemctl restart api.service",
	}}}); err != nil {
		t.Fatal(err)
	}
	file, _, err := config.LoadResources(home)
	if err != nil {
		t.Fatal(err)
	}
	m := Model{
		home:              home,
		width:             120,
		resourceHostIndex: 0,
		resourceKind:      resourceServices,
		resourceScope:     resourceScopeDiscovered,
		resourceFilter:    resourceFilterAll,
		resourceFile:      file,
		states: []hostState{{
			Host:           host.Host{Category: "prod", Name: "api-01"},
			ServiceDetails: []serviceDetail{{Unit: "api.service", Load: "loaded", Active: "active", Sub: "running", Managed: true}},
		}},
	}
	next, _ := m.startResourceCommandEdit()
	got := next.(Model)
	if got.mode != modeResourceCommandEdit {
		t.Fatalf("mode = %v, want command edit", got.mode)
	}
	got.resourceCommandField = 2
	got.resourceCommandCursor = len([]rune(got.resourceCommandForm.RestartCommand))
	got.resourceCommandForm.RestartCommand = "cd /data/api && ./restart.sh"
	if err := got.saveResourceCommandForm(); err != nil {
		t.Fatal(err)
	}
	file, _, err = config.LoadResources(home)
	if err != nil {
		t.Fatal(err)
	}
	if file.Items[0].RestartCommand != "cd /data/api && ./restart.sh" {
		t.Fatalf("restart command = %q", file.Items[0].RestartCommand)
	}
}

func TestResourceAddSavesEditedDefaultCommands(t *testing.T) {
	home := t.TempDir()
	m := Model{
		home:              home,
		width:             120,
		resourceHostIndex: 0,
		resourceKind:      resourceServices,
		states:            []hostState{{Host: host.Host{Category: "prod", Name: "api-01"}}},
	}
	next, _ := m.startResourceAdd()
	got := next.(Model)
	got.resourceAddName = "api.service"
	got.applyResourceAddDefaults()
	if got.resourceCommandForm.RestartCommand != "systemctl restart api.service" {
		t.Fatalf("restart default = %q", got.resourceCommandForm.RestartCommand)
	}
	got.resourceCommandForm.RestartCommand = "cd /data/api && ./restart.sh"
	next, _ = got.saveResourceAdd()
	saved := next.(Model)
	if saved.mode != modeResourceList {
		t.Fatalf("mode = %v, want list", saved.mode)
	}
	file, _, err := config.LoadResources(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Items) != 1 || file.Items[0].RestartCommand != "cd /data/api && ./restart.sh" {
		t.Fatalf("resources file = %#v", file)
	}
}

func TestResourceScopedLoadOnlyUpdatesRequestedKind(t *testing.T) {
	oldServiceAt := time.Now().Add(-time.Hour)
	m := Model{
		resourceKind:        resourceContainers,
		resourceServiceAt:   oldServiceAt,
		resourceContainerAt: time.Time{},
		states: []hostState{{
			ServiceDetails:   []serviceDetail{{Unit: "nginx.service", Load: "loaded", Active: "active", Sub: "running"}},
			ContainerDetails: []containerDetail{{Name: "old", Status: "Exited (0)"}},
		}},
	}
	next, _ := m.handleResourceLoad(resourceLoadMsg{
		Index:      0,
		Kind:       resourceContainers,
		Containers: []containerDetail{{Name: "api", Status: "Up 1 second"}},
	})
	got := next.(Model)
	if got.states[0].ContainerDetails[0].Name != "api" {
		t.Fatalf("container not updated: %+v", got.states[0].ContainerDetails)
	}
	if got.states[0].ServiceDetails[0].Unit != "nginx.service" {
		t.Fatalf("service should be preserved: %+v", got.states[0].ServiceDetails)
	}
	if !got.resourceServiceAt.Equal(oldServiceAt) || got.resourceContainerAt.IsZero() {
		t.Fatalf("timestamps service=%v container=%v", got.resourceServiceAt, got.resourceContainerAt)
	}
}

func TestStartResourceListShowsCachedContainersBeforeRefresh(t *testing.T) {
	home := t.TempDir()
	err := config.UpsertResourceContainerCache(home, "prod/api-01", []config.ResourceContainerCache{{
		Name:    "cached-api",
		Image:   "app:cached",
		Status:  "Up 5 minutes",
		Ports:   "80/tcp",
		CPU:     "0.10%",
		Memory:  "12MiB/1GiB",
		MemPerc: "1.2%",
	}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	m := Model{
		home: home,
		states: []hostState{{
			Host: host.Host{Category: "prod", Name: "api-01"},
		}},
	}
	next, cmd := m.startResourceList(0, resourceContainers, modeDashboard)
	got := next.(Model)
	if cmd == nil {
		t.Fatal("startResourceList should still refresh in background")
	}
	if len(got.states[0].ContainerDetails) != 1 || got.states[0].ContainerDetails[0].Name != "cached-api" {
		t.Fatalf("cached containers not shown immediately: %+v", got.states[0].ContainerDetails)
	}
	if !got.resourceLoading {
		t.Fatal("resource list should keep background loading after showing cache")
	}
}

func TestResourceContainerLoadWritesCache(t *testing.T) {
	home := t.TempDir()
	m := Model{
		home: home,
		states: []hostState{{
			Host: host.Host{Category: "prod", Name: "api-01"},
		}},
	}
	next, _ := m.handleResourceLoad(resourceLoadMsg{
		Index:      0,
		Kind:       resourceContainers,
		Containers: []containerDetail{{Name: "api", Image: "app:latest", Status: "Up 1 second", CPU: "0.1%"}},
	})
	got := next.(Model)
	if len(got.states[0].ContainerDetails) != 1 {
		t.Fatalf("containers = %+v", got.states[0].ContainerDetails)
	}
	file, _, err := config.LoadResourceCache(home)
	if err != nil {
		t.Fatal(err)
	}
	items, ok := config.ResourceContainerCacheForServer(file, "prod/api-01")
	if !ok || len(items) != 1 || items[0].Name != "api" {
		t.Fatalf("cache items = %#v ok=%v", items, ok)
	}
}

func TestResourceServiceLoadAlsoUpdatesPortsForServiceDiscovery(t *testing.T) {
	m := Model{
		resourceKind: resourceServices,
		states: []hostState{{
			ServiceDetails: []serviceDetail{{Unit: "old.service"}},
			PortDetails:    []portDetail{{Protocol: "tcp", Port: "22", PID: "1"}},
		}},
	}
	next, _ := m.handleResourceLoad(resourceLoadMsg{
		Index:    0,
		Kind:     resourceServices,
		Services: []serviceDetail{{Unit: "nginx.service", Load: "loaded", Active: "active", Sub: "running", MainPID: "100"}},
		Ports:    []portDetail{{Protocol: "tcp", Port: "80", PID: "100"}},
	})
	got := next.(Model)
	if got.states[0].ServiceDetails[0].Unit != "nginx.service" {
		t.Fatalf("service not updated: %+v", got.states[0].ServiceDetails)
	}
	if len(got.states[0].PortDetails) != 1 || got.states[0].PortDetails[0].Port != "80" {
		t.Fatalf("ports should update with service load: %+v", got.states[0].PortDetails)
	}
	if got.resourcePortAt.IsZero() {
		t.Fatal("port timestamp should update with service load")
	}
}

func TestResourceTabSwitchesTypeAndGSwitchesStatus(t *testing.T) {
	m := Model{resourceKind: resourceAll, resourceFilter: resourceFilterAll}
	next, _ := m.updateResourceList(tea.KeyMsg{Type: tea.KeyTab})
	got := next.(Model)
	if got.resourceKind != resourceContainers || got.resourceFilter != resourceFilterAll {
		t.Fatalf("after tab kind/filter = %v/%v, want containers/all", got.resourceKind, got.resourceFilter)
	}
	next, _ = got.updateResourceList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	got = next.(Model)
	if got.resourceFilter != resourceFilterRunning || got.resourceKind != resourceContainers {
		t.Fatalf("after g kind/filter = %v/%v, want containers/running", got.resourceKind, got.resourceFilter)
	}
}

func TestResourcePortGSwitchesScopeFilter(t *testing.T) {
	m := Model{
		width:              120,
		resourceHostIndex:  0,
		resourceKind:       resourcePorts,
		resourceFilter:     resourceFilterStopped,
		resourcePortFilter: resourcePortFilterAll,
		states: []hostState{{
			PortDetails: []portDetail{
				{Protocol: "tcp", Port: "22", LocalAddress: "0.0.0.0:22", Process: "sshd", PID: "1", Managed: true},
				{Protocol: "tcp", Port: "25", LocalAddress: "127.0.0.1:25", Process: "master", PID: "2", Managed: true},
				{Protocol: "tcp", Port: "3306", LocalAddress: "172.31.1.10:3306", Process: "mysqld", PID: "3", Managed: true},
				{Protocol: "tcp", Port: "1080", LocalAddress: "0.0.0.0:1080", Process: "docker-proxy", PID: "4", Container: "socks5", Managed: true},
			},
		}},
	}
	next, _ := m.updateResourceList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	got := next.(Model)
	if got.resourcePortFilter != resourcePortFilterPublic || got.resourceFilter != resourceFilterStopped {
		t.Fatalf("filters after g = port %v resource %v, want public and unchanged status", got.resourcePortFilter, got.resourceFilter)
	}
	indexes := got.filteredResourceIndexes()
	want := []resourceRef{{Kind: resourcePorts, Index: 0}, {Kind: resourcePorts, Index: 3}}
	if !reflect.DeepEqual(indexes, want) {
		t.Fatalf("public port indexes = %#v, want %#v", indexes, want)
	}
	got.resourcePortFilter = resourcePortFilterLoopback
	indexes = got.filteredResourceIndexes()
	want = []resourceRef{{Kind: resourcePorts, Index: 1}}
	if !reflect.DeepEqual(indexes, want) {
		t.Fatalf("loopback port indexes = %#v, want %#v", indexes, want)
	}
	got.resourcePortFilter = resourcePortFilterSpecific
	indexes = got.filteredResourceIndexes()
	want = []resourceRef{{Kind: resourcePorts, Index: 2}}
	if !reflect.DeepEqual(indexes, want) {
		t.Fatalf("specific port indexes = %#v, want %#v", indexes, want)
	}
	got.resourcePortFilter = resourcePortFilterContainer
	indexes = got.filteredResourceIndexes()
	want = []resourceRef{{Kind: resourcePorts, Index: 3}}
	if !reflect.DeepEqual(indexes, want) {
		t.Fatalf("container port indexes = %#v, want %#v", indexes, want)
	}
}

func TestResourceManagerPortsDeduplicatesProtocolPort(t *testing.T) {
	m := Model{
		width:             120,
		resourceHostIndex: 0,
		resourceAddKind:   resourcePorts,
		resourceFilter:    resourceFilterAll,
		states: []hostState{{
			PortDetails: []portDetail{
				{Protocol: "tcp", Port: "22", LocalAddress: "0.0.0.0:22", Process: "sshd", PID: "1"},
				{Protocol: "tcp", Port: "22", LocalAddress: ":::22", Process: "sshd", PID: "1"},
				{Protocol: "udp", Port: "323", LocalAddress: "127.0.0.1:323", Process: "chronyd", PID: "2"},
				{Protocol: "udp", Port: "323", LocalAddress: "::1:323", Process: "chronyd", PID: "2"},
			},
		}},
	}
	indexes := m.resourceManageDiscoveredRefs()
	want := []resourceRef{
		{Kind: resourcePorts, Index: 0},
		{Kind: resourcePorts, Index: 2},
	}
	if !reflect.DeepEqual(indexes, want) {
		t.Fatalf("port indexes = %#v, want deduplicated protocol/port refs %#v", indexes, want)
	}
}

func TestResourceDeleteIsDisabled(t *testing.T) {
	m := Model{
		width:             120,
		resourceHostIndex: 0,
		resourceKind:      resourceContainers,
		resourceIndex:     0,
		states: []hostState{{
			ContainerDetails: []containerDetail{{Name: "api", Status: "running", Managed: true}},
		}},
	}
	next, cmd := m.startResourceAction(resourceActionDelete)
	got := next.(Model)
	if cmd != nil {
		t.Fatal("resource delete should not run a command")
	}
	if got.mode == modeResourceConfirm {
		t.Fatal("resource delete should not open confirmation")
	}
	if !strings.Contains(got.status, "disabled") && !strings.Contains(got.status, "禁用") {
		t.Fatalf("status = %q, want delete disabled message", got.status)
	}
}

func TestManagedProcessCommandsEnableActions(t *testing.T) {
	m := Model{
		width:             120,
		resourceHostIndex: 0,
		resourceKind:      resourceProcesses,
		resourceScope:     resourceScopeDiscovered,
		resourceFilter:    resourceFilterAll,
		resourceFile: config.ResourcesFile{Items: []config.ManagedResource{{
			Server:         "prod/api-01",
			Kind:           config.ResourceKindProcess,
			Name:           "go-api",
			StartCommand:   "systemctl start go-api",
			StopCommand:    "systemctl stop go-api",
			RestartCommand: "systemctl restart go-api",
			LogCommand:     "journalctl -u go-api -n 200 --no-pager",
		}}},
		states: []hostState{{
			Host:        host.Host{Category: "prod", Name: "api-01"},
			PortDetails: []portDetail{{Protocol: "tcp", Port: "8080", LocalAddress: "0.0.0.0:8080", Process: "go-api", PID: "200", ProcessManaged: true}},
		}},
	}
	if resourceCommandFieldCount(resourceProcesses) != 4 {
		t.Fatalf("process command field count = %d, want 4", resourceCommandFieldCount(resourceProcesses))
	}
	next, cmd := m.startResourceAction(resourceActionRestart)
	got := next.(Model)
	if cmd != nil {
		t.Fatal("process confirm should not run command yet")
	}
	if got.mode != modeResourceConfirm || got.resourceActionResource != resourceProcesses {
		t.Fatalf("mode/action = %v/%v, want process confirm", got.mode, got.resourceActionResource)
	}
	if got.resourceActionScript(resourceProcesses, resourceActionRestart, "go-api") == "" {
		t.Fatal("managed process restart command should be available")
	}
	if got.resourceLogScript(resourceProcesses, "go-api", 200) == "" {
		t.Fatal("managed process log command should be available")
	}
}

func TestDiscoveredProcessActionsRequireConfiguredCommands(t *testing.T) {
	m := Model{
		width:             120,
		resourceHostIndex: 0,
		resourceKind:      resourceProcesses,
		resourceScope:     resourceScopeDiscovered,
		resourceFile: config.ResourcesFile{Items: []config.ManagedResource{{
			Server: "prod/api-01",
			Kind:   config.ResourceKindProcess,
			Name:   "go-api",
		}}},
		resourceFilter: resourceFilterAll,
		states: []hostState{{
			Host:        host.Host{Category: "prod", Name: "api-01"},
			PortDetails: []portDetail{{Protocol: "tcp", Port: "8080", LocalAddress: "0.0.0.0:8080", Process: "go-api", PID: "200", ProcessManaged: true}},
		}},
	}
	next, _ := m.startResourceAction(resourceActionRestart)
	got := next.(Model)
	if got.mode == modeResourceConfirm {
		t.Fatal("unconfigured process action should not open confirmation")
	}
	if !strings.Contains(got.status, "configure") && !strings.Contains(got.status, "配置") {
		t.Fatalf("status = %q, want configure prompt", got.status)
	}
	next, cmd := m.openResourceLog()
	got = next.(Model)
	if got.mode == modeResourceLog {
		t.Fatalf("unconfigured process log should not open log mode, mode=%v cmd=%v", got.mode, cmd)
	}
}

func TestResourceEditHelpOnlyShowsForManagedResource(t *testing.T) {
	m := Model{
		width:             120,
		resourceHostIndex: 0,
		resourceKind:      resourceProcesses,
		resourceScope:     resourceScopeDiscovered,
		resourceFilter:    resourceFilterAll,
		states: []hostState{{
			PortDetails: []portDetail{{Protocol: "tcp", Port: "8080", LocalAddress: "0.0.0.0:8080", Process: "go-api", PID: "200"}},
		}},
	}
	if strings.Contains(m.resourceListHelp(), "Edit e") || strings.Contains(m.resourceDetailHelp(), "Edit e") {
		t.Fatalf("unmanaged process help should not show edit: list=%q detail=%q", m.resourceListHelp(), m.resourceDetailHelp())
	}
	m.states[0].PortDetails[0].ProcessManaged = true
	if !strings.Contains(m.resourceListHelp(), "Edit e") || !strings.Contains(m.resourceDetailHelp(), "Edit e") {
		t.Fatalf("managed process help should show edit: list=%q detail=%q", m.resourceListHelp(), m.resourceDetailHelp())
	}
}

func TestResourceActionScriptsUseSudoFallback(t *testing.T) {
	dockerScript := resourceActionScript(resourceContainers, resourceActionRestart, "api")
	if !strings.Contains(dockerScript, "docker restart api") || !strings.Contains(dockerScript, "sudo -n docker restart api") {
		t.Fatalf("docker restart script missing sudo fallback:\n%s", dockerScript)
	}
	serviceScript := resourceActionScript(resourceServices, resourceActionRestart, "nginx.service")
	if !strings.Contains(serviceScript, "systemctl restart nginx.service") || !strings.Contains(serviceScript, "sudo -n systemctl restart nginx.service") {
		t.Fatalf("service restart script missing sudo fallback:\n%s", serviceScript)
	}
}

func TestResourceCardMovementUpdatesSelection(t *testing.T) {
	m := Model{
		width:             120,
		resourceHostIndex: 0,
		resourceKind:      resourceContainers,
		resourceView:      resourceViewCards,
		resourceIndex:     0,
		states: []hostState{{
			ContainerDetails: []containerDetail{
				{Name: "one", Status: "Up 1 second", Managed: true},
				{Name: "two", Status: "Up 1 second", Managed: true},
				{Name: "three", Status: "Up 1 second", Managed: true},
				{Name: "four", Status: "Up 1 second", Managed: true},
			},
		}},
	}
	m.moveResourceRight()
	if m.resourceIndex != 1 {
		t.Fatalf("right resourceIndex = %d, want 1", m.resourceIndex)
	}
	m.moveResourceLeft()
	if m.resourceIndex != 0 {
		t.Fatalf("left resourceIndex = %d, want 0", m.resourceIndex)
	}
	m.moveResourceDown()
	if m.resourceIndex != 3 {
		t.Fatalf("down resourceIndex = %d, want 3", m.resourceIndex)
	}
	m.moveResourceDown()
	if m.resourceIndex != 3 {
		t.Fatalf("down at bottom resourceIndex = %d, want clamp at 3", m.resourceIndex)
	}
}

func TestParsePortDetailsSSOutput(t *testing.T) {
	output := strings.Join([]string{
		`tcp LISTEN 0 4096 0.0.0.0:22 0.0.0.0:* users:(("sshd",pid=123,fd=3))`,
		`tcp LISTEN 0 4096 [::]:22 [::]:* users:(("sshd",pid=123,fd=4))`,
		`udp UNCONN 0 0 127.0.0.1:323 0.0.0.0:* users:(("chronyd",pid=456,fd=5))`,
		`udp UNCONN 0 0 [::1]:323 [::]:* users:(("chronyd",pid=456,fd=6))`,
		`tcp LISTEN 0 511 *:80 *:* users:(("nginx",pid=789,fd=6))`,
		`tcp LISTEN 0 4096 [::]:443 [::]:* users:(("caddy",pid=987,fd=4))`,
		`tcp 0 4096 0.0.0.0:8080 0.0.0.0:*`,
		`__SSHM_PORT_CGROUP__	789	nginx.service`,
	}, "\n")
	ports, errText := parsePortDetails(output)
	if errText != "" {
		t.Fatalf("errText = %q", errText)
	}
	if len(ports) != 5 {
		t.Fatalf("ports = %#v, want 5", ports)
	}
	if ports[0].Port != "22" || ports[0].LocalAddress != "0.0.0.0:22, [::]:22" || ports[0].State != "LISTEN" || ports[0].Process != "sshd" || ports[0].PID != "123" || ports[0].FD != "3, 4" {
		t.Fatalf("first port = %+v, want sshd on 22", ports[0])
	}
	if ports[3].Port != "443" || ports[3].Process != "caddy" || ports[3].PID != "987" {
		t.Fatalf("fourth port = %+v, want caddy on 443", ports[3])
	}
	var nginxPort portDetail
	for _, port := range ports {
		if port.Port == "80" {
			nginxPort = port
			break
		}
	}
	if nginxPort.Port != "80" || nginxPort.ServiceUnit != "nginx.service" {
		t.Fatalf("nginx port service unit = %+v, want nginx.service", nginxPort)
	}
	if ports[4].Port != "8080" || ports[4].Process != "" || ports[4].PID != "" {
		t.Fatalf("fifth port = %+v, want unnamed 8080", ports[4])
	}
}

func TestParsePortDetailsNetstatOutput(t *testing.T) {
	output := strings.Join([]string{
		`Proto Recv-Q Send-Q Local Address           Foreign Address         State       PID/Program name`,
		`tcp        0      0 0.0.0.0:22              0.0.0.0:*               LISTEN      123/sshd`,
		`tcp6       0      0 :::22                   :::*                    LISTEN      123/sshd`,
		`udp        0      0 127.0.0.1:323           0.0.0.0:*                           456/chronyd`,
		`udp6       0      0 ::1:323                 :::*                                456/chronyd`,
	}, "\n")
	ports, errText := parsePortDetails(output)
	if errText != "" {
		t.Fatalf("errText = %q", errText)
	}
	if len(ports) != 2 {
		t.Fatalf("ports = %#v, want 2", ports)
	}
	if ports[0].Protocol != "tcp" || ports[0].Port != "22" || ports[0].LocalAddress != "0.0.0.0:22, :::22" || ports[0].State != "LISTEN" || ports[0].Process != "sshd" || ports[0].PID != "123" {
		t.Fatalf("first port = %+v, want sshd on 22", ports[0])
	}
	if ports[1].Protocol != "udp" || ports[1].Port != "323" || ports[1].LocalAddress != "127.0.0.1:323, ::1:323" || ports[1].Process != "chronyd" || ports[1].PID != "456" {
		t.Fatalf("second port = %+v, want chronyd on 323", ports[1])
	}
}

func findLineContaining(view string, needle string) string {
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(ansi.Strip(line), needle) {
			return line
		}
	}
	return ""
}
