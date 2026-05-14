package tui

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/YaMaiDay/sshm/internal/config"
	"github.com/YaMaiDay/sshm/internal/host"
)

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
	lines := deploymentOutputLines(strings.Join([]string{
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
	for _, want := range []string{"✓ 更新前", "▶ 获取资源", "· 更新", "Cloning into '.'..."} {
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
	if !strings.Contains(view, "部署信息") || strings.Contains(view, "部署队列") || strings.Contains(view, "01") {
		t.Fatalf("single confirm should show deployment info, not queue row:\n%s", view)
	}
	if strings.Contains(view, "获取资源命令") || strings.Contains(view, "git pull --ff-only") {
		t.Fatalf("confirm view should not show resource command detail:\n%s", view)
	}
	if !strings.Contains(view, "获取资源") || !strings.Contains(view, "1步") {
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
	if !strings.Contains(view, "历史 50条") {
		t.Fatalf("confirm view missing history title:\n%s", view)
	}
	if got := strings.Count(view, "部署成功"); got != 50 {
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
	for _, want := range []string{"部署队列", "01", "web", "等待 3秒", "02", "api", "当前流程", "获取资源 1步", "更新 1步"} {
		if !strings.Contains(view, want) {
			t.Fatalf("confirm view missing %q:\n%s", want, view)
		}
	}
	for _, notWant := range []string{"历史 ", "服务器    prod/web", "应用      web", "    流程", "健康检查 1步"} {
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
	for _, want := range []string{"✓ 01", "✕ 02", "当前流程", "执行输出", "✕ 获取资源", "fatal: repository not found", "退出码 128", "重试失败 r", "重新部署 a"} {
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
	if !strings.Contains(got, "回滚失败") || !strings.Contains(got, "分钟前") {
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
