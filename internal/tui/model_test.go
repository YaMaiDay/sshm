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

	"github.com/YaMaiDay/sshm/internal/config"
	"github.com/YaMaiDay/sshm/internal/host"
	"github.com/YaMaiDay/sshm/internal/monitor"
	resourceservice "github.com/YaMaiDay/sshm/internal/resource"
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
		dashboard: dashboardState{Mode: dashboardCards},
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
		dashboard: dashboardState{Mode: dashboardCategory},
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
		dashboard: dashboardState{Mode: dashboardGrouped},
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
		dashboard: dashboardState{Mode: dashboardCards},
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
		Result: commandResult{Err: errors.New("git failed"), ExitCode: 128, Output: "fatal"},
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
	rows := containerDetailItemRows(m, resourceservice.ContainerDetail{
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

func TestServiceDetailRowsShowStatusAndDescription(t *testing.T) {
	m := Model{width: 120}
	rows := serviceDetailItemRows(m, resourceservice.ServiceDetail{
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
			ServiceDetails: []resourceservice.ServiceDetail{
				{Unit: "api.service", Load: "loaded", Active: "failed", Sub: "failed", Description: "API service"},
				{Unit: "worker.service", Load: "loaded", Active: "active", Sub: "running", Description: "Worker service"},
			},
			ContainerDetails: []resourceservice.ContainerDetail{
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

func TestContainerCPULimitText(t *testing.T) {
	m := Model{
		resourceState: resourceState{
			ContainerExtra: resourceservice.ContainerExtraDetail{NanoCpus: 1500000000},
		},
	}
	if got := m.containerCPULimitText(); got != "1.5 cores limit" {
		t.Fatalf("nano cpu limit = %q", got)
	}
	m = Model{
		resourceState: resourceState{
			ContainerExtra: resourceservice.ContainerExtraDetail{CpusetCpus: "0,1"},
		},
	}
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
		resourceState: resourceState{
			HostIndex: 0,
		},
		appConfig: config.AppConfig{Language: "zh"},
		states: []hostState{{
			ContainerDetails: []resourceservice.ContainerDetail{{
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
	if got := m.containerCardMeta(resourceservice.ContainerDetail{Status: "Up 4 weeks (healthy)"}); got != "28d" {
		t.Fatalf("container meta = %q, want 28d", got)
	}
	m.appConfig.Language = "zh"
	if got := m.containerCardMeta(resourceservice.ContainerDetail{Status: "Up 4 weeks (healthy)"}); got != "28天" {
		t.Fatalf("container zh meta = %q, want 28天", got)
	}
	if got := m.databaseCardMeta(resourceservice.DatabaseDetail{RawStatus: "Up 4 weeks (healthy)"}); got != "28天" {
		t.Fatalf("database zh meta = %q, want 28天", got)
	}
	if got := shortSystemdTimestampAge("bad timestamp", true); got != "" {
		t.Fatalf("bad systemd timestamp age = %q, want empty", got)
	}
}

func TestDatabaseCardTitleShowsEngineAndMetaShowsUptime(t *testing.T) {
	m := Model{
		resourceState: resourceState{
			HostIndex: 0,
			Kind:      resourceDatabases,
		},
		width: 120,
		states: []hostState{{
			DatabaseDetails: []resourceservice.DatabaseDetail{{
				Name:      "freedex",
				Engine:    "PostgreSQL",
				Status:    "running",
				RawStatus: "Up 2 hours",
				Managed:   true,
			}},
		}},
	}
	card := ansi.Strip(m.resourceCard(resourceRef{Kind: resourceDatabases, Index: 0}, false, 44))
	if !strings.Contains(card, "▤ [PostgreSQL] freedex") || strings.Contains(card, "[库]") {
		t.Fatalf("database card title should show engine and database name:\n%s", card)
	}
	if !strings.Contains(card, "2h") {
		t.Fatalf("database card should show uptime meta:\n%s", card)
	}
}

func TestResourceFiltersSeparateContainersAndServices(t *testing.T) {
	m := Model{
		resourceState: resourceState{
			HostIndex: 0,
			Kind:      resourceContainers,
			Filter:    resourceFilterProblems,
		},
		width: 120,
		states: []hostState{{
			ContainerDetails: []resourceservice.ContainerDetail{
				{Name: "api", Status: "Up 2 minutes", Managed: true},
				{Name: "worker", Status: "Restarting (1) 10 seconds ago", Managed: true},
			},
			ServiceDetails: []resourceservice.ServiceDetail{
				{Unit: "nginx.service", Load: "loaded", Active: "active", Sub: "running", Managed: true},
				{Unit: "redis.service", Load: "loaded", Active: "failed", Sub: "failed", Managed: true},
			},
		}},
	}
	containerIndexes := m.filteredResourceIndexes()
	if len(containerIndexes) != 1 || containerIndexes[0] != (resourceRef{Kind: resourceContainers, Index: 1}) {
		t.Fatalf("container problem indexes = %#v, want worker only", containerIndexes)
	}
	m.resourceState.Kind = resourceServices
	serviceIndexes := m.filteredResourceIndexes()
	if len(serviceIndexes) != 1 || serviceIndexes[0] != (resourceRef{Kind: resourceServices, Index: 1}) {
		t.Fatalf("service problem indexes = %#v, want redis only", serviceIndexes)
	}
}

func TestResourceAllIncludesContainersAndServices(t *testing.T) {
	m := Model{
		resourceState: resourceState{
			HostIndex: 0,
			Kind:      resourceAll,
			Filter:    resourceFilterAll,
		},
		width: 120,
		states: []hostState{{
			ContainerDetails: []resourceservice.ContainerDetail{{Name: "api", Status: "Up 2 minutes", Managed: true}},
			ServiceDetails:   []resourceservice.ServiceDetail{{Unit: "nginx.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/etc/systemd/system/nginx.service", ExecStart: "/usr/sbin/nginx", Managed: true}},
			PortDetails:      []resourceservice.PortDetail{{Protocol: "tcp", Port: "22", Process: "sshd", PID: "123", Managed: true}},
		}},
	}
	indexes := m.filteredResourceIndexes()
	want := []resourceRef{{Kind: resourceContainers, Index: 0}, {Kind: resourceServices, Index: 0}, {Kind: resourcePorts, Index: 0}}
	if !reflect.DeepEqual(indexes, want) {
		t.Fatalf("resource all indexes = %#v, want %#v", indexes, want)
	}
}

func TestResourceListShowsOnlyAddedResources(t *testing.T) {
	m := Model{
		resourceState: resourceState{
			HostIndex: 0,
			Kind:      resourceAll,
			Filter:    resourceFilterAll,
			File: config.ResourcesFile{Items: []config.ManagedResource{{
				Server: "prod/api-01",
				Kind:   config.ResourceKindContainer,
				Name:   "api",
				Added:  true,
			}}},
		},
		width: 120,
		states: []hostState{{
			Host: host.Host{Category: "prod", Name: "api-01"},
			ContainerDetails: []resourceservice.ContainerDetail{
				{Name: "api", Status: "Up 2 minutes"},
				{Name: "redis", Status: "Up 2 minutes"},
			},
			ServiceDetails: []resourceservice.ServiceDetail{{Unit: "nginx.service", Load: "loaded", Active: "active", Sub: "running"}},
		}},
	}
	indexes := m.filteredResourceIndexes()
	want := []resourceRef{{Kind: resourceContainers, Index: 0}}
	if !reflect.DeepEqual(indexes, want) {
		t.Fatalf("resource indexes = %#v, want only added resources %#v", indexes, want)
	}

	m.resourceState.AddKind = resourceContainers
	discovered := m.resourceManageDiscoveredRefs()
	want = []resourceRef{{Kind: resourceContainers, Index: 1}}
	if !reflect.DeepEqual(discovered, want) {
		t.Fatalf("discovered indexes = %#v, want only not-added resources %#v", discovered, want)
	}
}

func TestResourceServicesHideNotFoundInactiveDeadUnits(t *testing.T) {
	m := Model{
		resourceState: resourceState{
			HostIndex: 0,
			Kind:      resourceServices,
			Filter:    resourceFilterAll,
		},
		width: 120,
		states: []hostState{{
			ServiceDetails: []resourceservice.ServiceDetail{
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
		resourceState: resourceState{
			HostIndex: 0,
			AddKind:   resourceServices,
			Filter:    resourceFilterAll,
		},
		width: 120,
		states: []hostState{{
			ServiceDetails: []resourceservice.ServiceDetail{
				{Unit: "sshd.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/usr/lib/systemd/system/sshd.service", ExecStart: "/usr/sbin/sshd -D"},
				{Unit: "api.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/etc/systemd/system/api.service", ExecStart: "/data/api/server"},
				{Unit: "worker.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/usr/lib/systemd/system/worker.service", WorkingDirectory: "/opt/worker"},
				{Unit: "broken.service", Load: "loaded", Active: "failed", Sub: "failed", FragmentPath: "/usr/lib/systemd/system/broken.service"},
			},
		}},
	}
	indexes := m.resourceManageDiscoveredRefs()
	want := []resourceRef{
		{Kind: resourceServices, Index: 3},
		{Kind: resourceServices, Index: 1},
		{Kind: resourceServices, Index: 0},
		{Kind: resourceServices, Index: 2},
	}
	if !reflect.DeepEqual(indexes, want) {
		t.Fatalf("discovered service indexes = %#v, want %#v", indexes, want)
	}
}

func TestResourceManagerServicesHideNotFoundUnlessSearching(t *testing.T) {
	m := Model{
		resourceState: resourceState{
			HostIndex: 0,
			AddKind:   resourceServices,
			Filter:    resourceFilterAll,
		},
		width: 120,
		states: []hostState{{
			ServiceDetails: []resourceservice.ServiceDetail{
				{Unit: "iptables.service", Load: "not-found", Active: "inactive", Sub: "dead"},
				{Unit: "nginx.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/usr/lib/systemd/system/nginx.service"},
			},
		}},
	}
	indexes := m.resourceManageDiscoveredRefs()
	want := []resourceRef{{Kind: resourceServices, Index: 1}}
	if !reflect.DeepEqual(indexes, want) {
		t.Fatalf("discovered services = %#v, want %#v", indexes, want)
	}
	m.resourceState.ManageQuery = "iptables"
	indexes = m.resourceManageDiscoveredRefs()
	want = []resourceRef{{Kind: resourceServices, Index: 0}}
	if !reflect.DeepEqual(indexes, want) {
		t.Fatalf("searched services = %#v, want %#v", indexes, want)
	}
}

func TestResourceManagerSortsDiscoveredByStatusAndName(t *testing.T) {
	m := Model{
		resourceState: resourceState{
			HostIndex: 0,
			AddKind:   resourceServices,
			Filter:    resourceFilterAll,
		},
		width: 120,
		states: []hostState{{
			ServiceDetails: []resourceservice.ServiceDetail{
				{Unit: "z-running.service", Load: "loaded", Active: "active", Sub: "running"},
				{Unit: "b-failed.service", Load: "loaded", Active: "failed", Sub: "failed"},
				{Unit: "a-failed.service", Load: "loaded", Active: "failed", Sub: "failed"},
				{Unit: "a-running.service", Load: "loaded", Active: "active", Sub: "running"},
				{Unit: "c-stopped.service", Load: "loaded", Active: "inactive", Sub: "dead"},
				{Unit: "b-active.service", Load: "loaded", Active: "active", Sub: "exited"},
			},
		}},
	}
	indexes := m.resourceManageDiscoveredRefs()
	want := []resourceRef{
		{Kind: resourceServices, Index: 2},
		{Kind: resourceServices, Index: 1},
		{Kind: resourceServices, Index: 3},
		{Kind: resourceServices, Index: 0},
		{Kind: resourceServices, Index: 5},
		{Kind: resourceServices, Index: 4},
	}
	if !reflect.DeepEqual(indexes, want) {
		t.Fatalf("sorted discovered refs = %#v, want %#v", indexes, want)
	}
}

func TestResourceManagerSortsAddedByStatusAndName(t *testing.T) {
	m := Model{
		resourceState: resourceState{
			HostIndex: 0,
			AddKind:   resourceServices,
			File: config.ResourcesFile{Items: []config.ManagedResource{
				{Server: "prod/api-01", Kind: config.ResourceKindService, Name: "z-running.service", Added: true},
				{Server: "prod/api-01", Kind: config.ResourceKindService, Name: "b-failed.service", Added: true},
				{Server: "prod/api-01", Kind: config.ResourceKindService, Name: "missing.service", Added: true},
				{Server: "prod/api-01", Kind: config.ResourceKindService, Name: "a-running.service", Added: true},
				{Server: "prod/api-01", Kind: config.ResourceKindService, Name: "b-active.service", Added: true},
			}},
		},
		width: 120,
		states: []hostState{{
			Host: host.Host{Category: "prod", Name: "api-01"},
			ServiceDetails: []resourceservice.ServiceDetail{
				{Unit: "z-running.service", Load: "loaded", Active: "active", Sub: "running"},
				{Unit: "b-failed.service", Load: "loaded", Active: "failed", Sub: "failed"},
				{Unit: "a-running.service", Load: "loaded", Active: "active", Sub: "running"},
				{Unit: "b-active.service", Load: "loaded", Active: "active", Sub: "exited"},
			},
		}},
	}
	items := m.resourceManageFavorites()
	got := []string{}
	for _, item := range items {
		got = append(got, item.Name)
	}
	want := []string{"b-failed.service", "missing.service", "a-running.service", "z-running.service", "b-active.service"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sorted added names = %#v, want %#v", got, want)
	}
}

func TestResourceManagerServiceLineShowsLocalizedStatusAndRawState(t *testing.T) {
	m := Model{
		resourceState: resourceState{
			HostIndex: 0,
		},
		appConfig: config.AppConfig{Language: "zh"},
		states: []hostState{{
			ServiceDetails: []resourceservice.ServiceDetail{{Unit: "api.service", Load: "loaded", Active: "active", Sub: "running"}},
		}},
	}
	line := ansi.Strip(m.resourceManageRefLine(resourceRef{Kind: resourceServices, Index: 0}, true, 80))
	if !strings.Contains(line, "运行") || !strings.Contains(line, "api.service") || !strings.Contains(line, "loaded active/running") {
		t.Fatalf("service manager line should include localized status, name and raw state:\n%s", line)
	}
}

func TestResourceManagerContainerLineShowsRawStatus(t *testing.T) {
	m := Model{
		resourceState: resourceState{
			HostIndex: 0,
		},
		appConfig: config.AppConfig{Language: "zh"},
		states: []hostState{{
			ContainerDetails: []resourceservice.ContainerDetail{{Name: "postgres", Status: "Up 2 hours (healthy)"}},
		}},
	}
	line := ansi.Strip(m.resourceManageRefLine(resourceRef{Kind: resourceContainers, Index: 0}, true, 80))
	if !strings.Contains(line, "健康") || !strings.Contains(line, "postgres") || !strings.Contains(line, "Up 2 hours") {
		t.Fatalf("container manager line should include localized status, name and raw status:\n%s", line)
	}
}

func TestResourceManagerStatusColumnAlignsNames(t *testing.T) {
	m := Model{
		resourceState: resourceState{
			HostIndex: 0,
		},
		appConfig: config.AppConfig{Language: "zh"},
		states: []hostState{{
			ServiceDetails: []resourceservice.ServiceDetail{
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
	if strings.Index(line1, "loaded active/running") != strings.Index(line2, "loaded failed/failed") {
		t.Fatalf("resource manager raw state column should align:\n%s\n%s", line1, line2)
	}
}

func TestResourceManagerAddedLineMatchesDiscoveredLine(t *testing.T) {
	m := Model{
		resourceState: resourceState{
			HostIndex: 0,
		},
		appConfig: config.AppConfig{Language: "zh"},
		states: []hostState{{
			ServiceDetails: []resourceservice.ServiceDetail{{Unit: "api.service", Load: "loaded", Active: "failed", Sub: "failed"}},
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
		resourceState: resourceState{
			HostIndex: 0,
			AddKind:   resourceServices,
			Filter:    resourceFilterAll,
		},
		width: 120,
		states: []hostState{{
			ServiceDetails: []resourceservice.ServiceDetail{
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
		{Kind: resourceServices, Index: 6},
		{Kind: resourceServices, Index: 8},
		{Kind: resourceServices, Index: 0},
		{Kind: resourceServices, Index: 7},
		{Kind: resourceServices, Index: 4},
		{Kind: resourceServices, Index: 3},
		{Kind: resourceServices, Index: 1},
		{Kind: resourceServices, Index: 5},
		{Kind: resourceServices, Index: 2},
	}
	if !reflect.DeepEqual(indexes, want) {
		t.Fatalf("discovered service indexes = %#v, want %#v", indexes, want)
	}
}

func TestResourceServicesHideSystemHelpersFromRealServerSet(t *testing.T) {
	m := Model{
		resourceState: resourceState{
			HostIndex: 0,
			AddKind:   resourceServices,
			Filter:    resourceFilterAll,
		},
		width: 120,
		states: []hostState{{
			ServiceDetails: []resourceservice.ServiceDetail{
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
		resourceState: resourceState{
			HostIndex: 0,
			AddKind:   resourceServices,
			Filter:    resourceFilterAll,
		},
		width: 120,
		states: []hostState{{
			ServiceDetails: []resourceservice.ServiceDetail{
				{Unit: "nginx.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/usr/lib/systemd/system/nginx.service", ExecStart: "/usr/sbin/nginx -g 'daemon off;'", MainPID: "100"},
				{Unit: "sshd.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/usr/lib/systemd/system/sshd.service", ExecStart: "/usr/sbin/sshd -D", MainPID: "200"},
				{Unit: "x-ui.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/usr/lib/systemd/system/x-ui.service", ExecStart: "/usr/local/x-ui/x-ui", MainPID: "300"},
				{Unit: "chronyd.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/usr/lib/systemd/system/chronyd.service", ExecStart: "/usr/sbin/chronyd", MainPID: "400"},
			},
			PortDetails: []resourceservice.PortDetail{
				{Protocol: "tcp", Port: "80", LocalAddress: "0.0.0.0:80", Process: "nginx", PID: "100", Count: 1},
				{Protocol: "tcp", Port: "22", LocalAddress: "0.0.0.0:22", Process: "sshd", PID: "200", Count: 1},
				{Protocol: "tcp", Port: "2053", LocalAddress: "*:2053", Process: "x-ui", PID: "301", Count: 1},
				{Protocol: "udp", Port: "323", LocalAddress: "127.0.0.1:323", Process: "chronyd", PID: "400", Count: 1},
			},
		}},
	}
	indexes := m.resourceManageDiscoveredRefs()
	want := []resourceRef{
		{Kind: resourceServices, Index: 3},
		{Kind: resourceServices, Index: 0},
		{Kind: resourceServices, Index: 1},
		{Kind: resourceServices, Index: 2},
	}
	if !reflect.DeepEqual(indexes, want) {
		t.Fatalf("service indexes = %#v, want all discovered services", indexes)
	}
}

func TestResourceProcessesShowStandaloneListenersOnly(t *testing.T) {
	m := Model{
		resourceState: resourceState{
			HostIndex: 0,
			AddKind:   resourceProcesses,
			Filter:    resourceFilterAll,
		},
		width: 120,
		states: []hostState{{
			ServiceDetails: []resourceservice.ServiceDetail{
				{Unit: "nginx.service", Load: "loaded", Active: "active", Sub: "running", ExecStart: "/usr/sbin/nginx", MainPID: "100"},
			},
			PortDetails: []resourceservice.PortDetail{
				{Protocol: "tcp", Port: "80", LocalAddress: "0.0.0.0:80", Process: "nginx", PID: "100", Count: 1},
				{Protocol: "tcp", Port: "8080", LocalAddress: "0.0.0.0:8080", Process: "go-api", PID: "200", Count: 1},
				{Protocol: "tcp", Port: "1080", LocalAddress: "0.0.0.0:1080", Process: "docker-proxy", PID: "300", Container: "socks5", Count: 1},
				{Protocol: "tcp", Port: "22", LocalAddress: "0.0.0.0:22", Process: "sshd", PID: "400", Count: 1},
			},
		}},
	}
	indexes := m.resourceManageDiscoveredRefs()
	want := []resourceRef{
		{Kind: resourceProcesses, Index: 2},
		{Kind: resourceProcesses, Index: 1},
		{Kind: resourceProcesses, Index: 0},
		{Kind: resourceProcesses, Index: 3},
	}
	if !reflect.DeepEqual(indexes, want) {
		t.Fatalf("process indexes = %#v, want all discovered processes", indexes)
	}
}

func TestResourceProcessesHideCgroupOwnedListeners(t *testing.T) {
	m := Model{
		resourceState: resourceState{
			HostIndex: 0,
			AddKind:   resourceProcesses,
			Filter:    resourceFilterAll,
		},
		width: 120,
		states: []hostState{{
			ServiceDetails: []resourceservice.ServiceDetail{
				{Unit: "x-ui.service", Load: "loaded", Active: "active", Sub: "running"},
			},
			PortDetails: []resourceservice.PortDetail{
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
		resourceState: resourceState{
			HostIndex: 0,
			Kind:      resourceProcesses,
			Scope:     resourceScopeDiscovered,
			Filter:    resourceFilterAll,
			File: config.ResourcesFile{Items: []config.ManagedResource{
				{Server: "prod/api-01", Kind: config.ResourceKindProcess, Name: "go-api", Added: true},
			}},
		},
		width: 120,
		states: []hostState{{
			Host:        host.Host{Category: "prod", Name: "api-01"},
			PortDetails: []resourceservice.PortDetail{{Protocol: "tcp", Port: "8080", LocalAddress: "0.0.0.0:8080", Process: "go-api", PID: "200", Count: 1}},
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
		resourceState: resourceState{
			HostIndex: 0,
			Kind:      resourceProcesses,
			Scope:     resourceScopeDiscovered,
			Filter:    resourceFilterAll,
			File: config.ResourcesFile{Items: []config.ManagedResource{
				{Server: "prod/api-01", Kind: config.ResourceKindProcess, Name: "go-api", Added: true},
			}},
		},
		width: 120,
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
		resourceState: resourceState{
			HostIndex: 0,
			Kind:      resourceServices,
			Scope:     resourceScopeDiscovered,
			Filter:    resourceFilterAll,
			File: config.ResourcesFile{Items: []config.ManagedResource{
				{Server: "prod/api-01", Kind: config.ResourceKindService, Name: "api.service", Added: true},
			}},
		},
		width:  120,
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
		resourceState: resourceState{
			HostIndex: 0,
			Kind:      resourceContainers,
			Scope:     resourceScopeDiscovered,
			Filter:    resourceFilterAll,
			File: config.ResourcesFile{Items: []config.ManagedResource{{
				Server: "prod/api-01",
				Kind:   config.ResourceKindContainer,
				Name:   "api",
				Added:  true,
			}}},
		},
		home:  home,
		width: 120,
		states: []hostState{{
			Host:             host.Host{Category: "prod", Name: "api-01"},
			ContainerDetails: []resourceservice.ContainerDetail{{Name: "api", Status: "Up 1 second", Managed: true}},
		}},
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
	if len(file.Items) != 1 || file.Items[0].Favorite {
		t.Fatalf("resources file after unfavorite = %#v", file)
	}
}

func TestResourceManagerAddsAndRemovesFavorite(t *testing.T) {
	home := t.TempDir()
	m := Model{
		resourceState: resourceState{
			HostIndex: 0,
			Kind:      resourceServices,
			Scope:     resourceScopeDiscovered,
			Filter:    resourceFilterAll,
		},
		home:  home,
		width: 120,
		states: []hostState{{
			Host:           host.Host{Category: "prod", Name: "api-01"},
			ServiceDetails: []resourceservice.ServiceDetail{{Unit: "api.service", Load: "loaded", Active: "active", Sub: "running", FragmentPath: "/etc/systemd/system/api.service"}},
		}},
	}
	next, _ := m.startResourceAdd()
	got := next.(Model)
	if got.mode != modeResourceAdd || got.resourceState.AddKind != resourceServices {
		t.Fatalf("mode/kind = %v/%v, want resource manager services", got.mode, got.resourceState.AddKind)
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
	if got.resourceState.ManagePane != 0 {
		t.Fatalf("pane after add = %d, want discovered", got.resourceState.ManagePane)
	}
	got.resourceState.ManagePane = 1
	next, _ = got.updateResourceAdd(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
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
		Added:  true,
	}}}); err != nil {
		t.Fatal(err)
	}
	file, _, err := config.LoadResources(home)
	if err != nil {
		t.Fatal(err)
	}
	m := Model{
		resourceState: resourceState{
			HostIndex: 0,
			Kind:      resourceServices,
			Filter:    resourceFilterAll,
			File:      file,
		},
		home:  home,
		width: 120,
		states: []hostState{{
			Host:           host.Host{Category: "prod", Name: "api-01"},
			ServiceDetails: []resourceservice.ServiceDetail{{Unit: "api.service", Load: "loaded", Active: "active", Sub: "running", Managed: true}},
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

func TestResourceListXCanRemoveDockerResourceAfterConfirmation(t *testing.T) {
	home := t.TempDir()
	if err := config.SaveResources(home, config.ResourcesFile{Items: []config.ManagedResource{{
		Server:   "prod/api-01",
		Kind:     config.ResourceKindContainer,
		Name:     "api",
		Added:    true,
		Favorite: true,
	}}}); err != nil {
		t.Fatal(err)
	}
	file, _, err := config.LoadResources(home)
	if err != nil {
		t.Fatal(err)
	}
	m := Model{
		resourceState: resourceState{
			HostIndex: 0,
			Kind:      resourceContainers,
			Filter:    resourceFilterAll,
			File:      file,
		},
		home:  home,
		width: 120,
		states: []hostState{{
			Host:             host.Host{Category: "prod", Name: "api-01"},
			ContainerDetails: []resourceservice.ContainerDetail{{Name: "api", Status: "Up 1 second", Managed: true, Favorite: true}},
		}},
	}
	next, _ := m.updateResourceList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	got := next.(Model)
	if got.mode != modeConfirmAction || got.confirm.Kind != confirmRemoveResource {
		t.Fatalf("mode/confirm = %v/%v, want remove confirmation", got.mode, got.confirm.Kind)
	}
	next, _ = got.updateConfirmAction(tea.KeyMsg{Type: tea.KeyEnter})
	file, _, err = config.LoadResources(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Items) != 0 {
		t.Fatalf("docker resource should be removed from sshm config: %#v", file.Items)
	}
}

func TestResourceActionShortcutsAreConsistentOnListAndDetail(t *testing.T) {
	base := Model{
		resourceState: resourceState{
			HostIndex: 0,
			Kind:      resourceServices,
			Filter:    resourceFilterAll,
		},
		width: 120,
		states: []hostState{{
			Host:           host.Host{Category: "prod", Name: "api-01"},
			ServiceDetails: []resourceservice.ServiceDetail{{Unit: "api.service", Load: "loaded", Active: "active", Sub: "running", Managed: true}},
		}},
	}

	for key, action := range map[rune]resourceActionKind{
		's': resourceActionStart,
		'p': resourceActionStop,
		'c': resourceActionRestart,
	} {
		next, _ := base.updateResourceList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})
		got := next.(Model)
		if got.mode != modeResourceConfirm || got.resourceState.Action != action {
			t.Fatalf("list key %q mode/action = %v/%v, want confirm/%v", key, got.mode, got.resourceState.Action, action)
		}

		detail := base
		detail.mode = modeResourceDetail
		next, _ = detail.updateResourceDetail(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})
		got = next.(Model)
		if got.mode != modeResourceConfirm || got.resourceState.Action != action {
			t.Fatalf("detail key %q mode/action = %v/%v, want confirm/%v", key, got.mode, got.resourceState.Action, action)
		}
	}
}

func TestResourceDetailRRefreshesInsteadOfRestarting(t *testing.T) {
	m := Model{
		resourceState: resourceState{
			HostIndex: 0,
			Kind:      resourceServices,
			Filter:    resourceFilterAll,
		},
		width: 120,
		mode:  modeResourceDetail,
		states: []hostState{{
			Host:           host.Host{Category: "prod", Name: "api-01"},
			ServiceDetails: []resourceservice.ServiceDetail{{Unit: "api.service", Load: "loaded", Active: "active", Sub: "running", Managed: true}},
		}},
	}
	next, _ := m.updateResourceDetail(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	got := next.(Model)
	if got.mode == modeResourceConfirm || got.resourceState.Action == resourceActionRestart {
		t.Fatalf("detail r should refresh, not restart: mode/action=%v/%v", got.mode, got.resourceState.Action)
	}
	if !got.resourceState.Loading || got.resourceState.LoadingKind != resourceServices {
		t.Fatalf("detail r should start resource refresh: loading=%v kind=%v", got.resourceState.Loading, got.resourceState.LoadingKind)
	}
}

func TestResourceDetailScrollClampsBeforeMoving(t *testing.T) {
	name := "postgres"
	m := Model{
		resourceState: resourceState{
			HostIndex: 0,
			Kind:      resourceDatabases,
			Filter:    resourceFilterAll,
			File: config.ResourcesFile{Items: []config.ManagedResource{{
				Server:   "prod/api-01",
				Kind:     config.ResourceKindDatabase,
				Name:     name,
				Added:    true,
				DBEngine: "PostgreSQL",
				DBHost:   "127.0.0.1",
				DBPort:   "5432",
				DBName:   name,
			}}},
			DatabaseExtraName: name,
			DatabaseExtra: resourceservice.DatabaseExtraDetail{
				Version:         "PostgreSQL 16",
				SizeBytes:       72 * 1024 * 1024 * 1024,
				TotalBytes:      999 * 1024 * 1024 * 1024,
				DBTotalBytes:    73 * 1024 * 1024 * 1024,
				IndexTotalBytes: 23 * 1024 * 1024 * 1024,
				TableTop: []resourceservice.DatabaseTableSize{
					{Name: "public.a", Size: 26 * 1024 * 1024 * 1024},
					{Name: "public.b", Size: 23 * 1024 * 1024 * 1024},
					{Name: "public.c", Size: 19 * 1024 * 1024 * 1024},
					{Name: "public.d", Size: 1 * 1024 * 1024 * 1024},
					{Name: "public.e", Size: 1 * 1024 * 1024 * 1024},
				},
				Connections:    "26",
				MaxConnections: "100",
				ActiveConns:    "3",
				IdleConns:      "16",
				CacheHit:       "95.34%",
				LockWaits:      "0",
				LongTx:         "0",
				Deadlocks:      "31",
			},
		},
		width:  120,
		height: 10,
		mode:   modeResourceDetail,
		states: []hostState{{
			Host: host.Host{Category: "prod", Name: "api-01"},
			DatabaseDetails: []resourceservice.DatabaseDetail{{
				Name:    name,
				Engine:  "PostgreSQL",
				Status:  "running",
				Managed: true,
			}},
		}},
	}
	maxScroll := m.resourceDetailMaxScroll()
	if maxScroll <= 0 {
		t.Fatalf("maxScroll = %d, want scrollable detail", maxScroll)
	}
	m.resourceState.Scroll = maxScroll + 100
	got := m.moveResourceDetailScroll(-3)
	if got.resourceState.Scroll != maxInt(0, maxScroll-3) {
		t.Fatalf("scroll after up from overflow = %d, want %d", got.resourceState.Scroll, maxInt(0, maxScroll-3))
	}
}

func TestResourceListTPinsDiscoveredContainer(t *testing.T) {
	home := t.TempDir()
	m := Model{
		resourceState: resourceState{
			HostIndex: 0,
			Kind:      resourceContainers,
			Filter:    resourceFilterAll,
			File: config.ResourcesFile{Items: []config.ManagedResource{{
				Server: "prod/api-01",
				Kind:   config.ResourceKindContainer,
				Name:   "api",
				Added:  true,
			}}},
		},
		home:  home,
		width: 120,
		states: []hostState{{
			Host:             host.Host{Category: "prod", Name: "api-01"},
			ContainerDetails: []resourceservice.ContainerDetail{{Name: "api", Status: "Up 1 second"}},
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
		resourceState: resourceState{
			HostIndex: 0,
			Kind:      resourceContainers,
			Filter:    resourceFilterAll,
			Sort:      resourceSortCPU,
			File: config.ResourcesFile{Items: []config.ManagedResource{
				{Server: "prod/api-01", Kind: config.ResourceKindContainer, Name: "api", Added: true, Pinned: true, PinnedOrder: 1},
				{Server: "prod/api-01", Kind: config.ResourceKindContainer, Name: "db", Added: true},
				{Server: "prod/api-01", Kind: config.ResourceKindContainer, Name: "web", Added: true},
			}},
		},
		width: 120,
		states: []hostState{{
			Host: host.Host{Category: "prod", Name: "api-01"},
			ContainerDetails: []resourceservice.ContainerDetail{
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

func TestResourceAllSortsPinnedBeforeKindGroups(t *testing.T) {
	m := Model{
		resourceState: resourceState{
			HostIndex: 0,
			Kind:      resourceAll,
			Filter:    resourceFilterAll,
			File: config.ResourcesFile{Items: []config.ManagedResource{
				{Server: "prod/api-01", Kind: config.ResourceKindContainer, Name: "api", Added: true},
				{Server: "prod/api-01", Kind: config.ResourceKindDatabase, Name: "freedex", Added: true, Pinned: true, PinnedOrder: 1, DBEngine: "PostgreSQL", DBHost: "127.0.0.1", DBPort: "5432", DBName: "freedex"},
			}},
		},
		width: 120,
		states: []hostState{{
			Host:             host.Host{Category: "prod", Name: "api-01"},
			ContainerDetails: []resourceservice.ContainerDetail{{Name: "api", Status: "Up 1 second"}},
			DatabaseDetails: []resourceservice.DatabaseDetail{{
				Name:       "freedex",
				Engine:     "PostgreSQL",
				Configured: true,
				Managed:    true,
			}},
		}},
	}
	refs := m.filteredResourceIndexes()
	if len(refs) < 2 {
		t.Fatalf("refs = %#v, want at least 2 refs", refs)
	}
	if refs[0].Kind != resourceDatabases {
		t.Fatalf("first ref kind = %v, want database pinned before kind groups", refs[0].Kind)
	}
}

func TestResourceErrorTextHidesRawSSHExit255(t *testing.T) {
	m := Model{
		resourceState: resourceState{
			HostIndex: 0,
			Kind:      resourceAll,
		},
		states: []hostState{{
			ServiceError:     "exit status 255",
			ContainerError:   "exit status 255",
			PortDetailsError: "exit status 255",
		}},
	}
	got := m.resourceErrorText()
	if strings.Contains(got, "exit status 255") || got == "" {
		t.Fatalf("resource error text = %q, want friendly ssh error", got)
	}
	if strings.Count(got, "SSH") != 1 {
		t.Fatalf("resource error text = %q, want deduplicated SSH error", got)
	}
}

func TestResourceSortShortcutCyclesSortMode(t *testing.T) {
	m := Model{
		resourceState: resourceState{
			HostIndex: 0,
			Kind:      resourceContainers,
			Filter:    resourceFilterAll,
		},
		width: 120,
		states: []hostState{{
			Host:             host.Host{Category: "prod", Name: "api-01"},
			ContainerDetails: []resourceservice.ContainerDetail{{Name: "api", Status: "Up 1 second"}},
		}},
	}
	next, _ := m.updateResourceList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	got := next.(Model)
	if got.resourceState.Sort != resourceSortStatus || !strings.Contains(got.status, "Sort") {
		t.Fatalf("sort after y = %v status=%q, want status sort", got.resourceState.Sort, got.status)
	}
}

func TestResourceManagerXCanRemoveDockerResourceAfterConfirmation(t *testing.T) {
	home := t.TempDir()
	file := config.ResourcesFile{Items: []config.ManagedResource{{
		Server:   "prod/api-01",
		Kind:     config.ResourceKindContainer,
		Name:     "api",
		Added:    true,
		Favorite: true,
	}}}
	if err := config.SaveResources(home, file); err != nil {
		t.Fatal(err)
	}
	m := Model{
		resourceState: resourceState{
			HostIndex:           0,
			AddKind:             resourceContainers,
			ManagePane:          1,
			ManageFavoriteIndex: 0,
			File:                file,
		},
		home:  home,
		width: 120,
		states: []hostState{{
			Host:             host.Host{Category: "prod", Name: "api-01"},
			ContainerDetails: []resourceservice.ContainerDetail{{Name: "api", Status: "Up 1 second", Managed: true, Favorite: true}},
		}},
	}
	next, _ := m.updateResourceAdd(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	got := next.(Model)
	if got.mode == modeConfirmAction {
		t.Fatalf("resource manager docker remove should not ask confirmation")
	}
	saved, _, err := config.LoadResources(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Items) != 0 {
		t.Fatalf("docker resource should be removed from sshm config: %#v", saved.Items)
	}
}

func TestResourceManagerSelectionDoesNotWrap(t *testing.T) {
	m := Model{
		resourceState: resourceState{
			HostIndex:  0,
			AddKind:    resourceServices,
			ManagePane: 0,
		},
		states: []hostState{{
			ServiceDetails: []resourceservice.ServiceDetail{
				{Unit: "a.service", Load: "loaded", Active: "active", Sub: "running"},
				{Unit: "b.service", Load: "loaded", Active: "active", Sub: "running"},
			},
		}},
	}
	m.moveResourceManageSelection(-1)
	if m.resourceState.ManageDiscoveredIndex != 0 {
		t.Fatalf("discovered index after up at top = %d, want 0", m.resourceState.ManageDiscoveredIndex)
	}
	m.moveResourceManageSelection(1)
	m.moveResourceManageSelection(1)
	if m.resourceState.ManageDiscoveredIndex != 1 {
		t.Fatalf("discovered index after down past bottom = %d, want 1", m.resourceState.ManageDiscoveredIndex)
	}
	m.resourceState.ManagePane = 1
	m.resourceState.ManageFavoriteIndex = 0
	m.resourceState.File = config.ResourcesFile{Items: []config.ManagedResource{
		{Server: "prod/api-01", Kind: config.ResourceKindService, Name: "a.service", Added: true},
		{Server: "prod/api-01", Kind: config.ResourceKindService, Name: "b.service", Added: true},
	}}
	m.states[0].Host = host.Host{Category: "prod", Name: "api-01"}
	m.moveResourceManageSelection(-1)
	if m.resourceState.ManageFavoriteIndex != 0 {
		t.Fatalf("favorite index after up at top = %d, want 0", m.resourceState.ManageFavoriteIndex)
	}
	m.moveResourceManageSelection(1)
	m.moveResourceManageSelection(1)
	if m.resourceState.ManageFavoriteIndex != 1 {
		t.Fatalf("favorite index after down past bottom = %d, want 1", m.resourceState.ManageFavoriteIndex)
	}
}

func TestResourceLogScrollUsesViewportWindow(t *testing.T) {
	m := Model{
		resourceState: resourceState{
			LogOutput: strings.Join([]string{"l1", "l2", "l3", "l4", "l5", "l6", "l7"}, "\n"),
		}, height: 8}
	next, _ := m.updateResourceLog(tea.KeyMsg{Type: tea.KeyDown})
	got := next.(Model)
	if got.resourceState.LogScroll != 1 {
		t.Fatalf("scroll after down = %d, want 1", got.resourceState.LogScroll)
	}
	start, end := resourceLogWindowRange(len(got.resourceLogLines()), got.resourceState.LogScroll, got.resourceLogBodyHeight())
	if start != 1 || end != 5 {
		t.Fatalf("window after down = %d-%d, want 1-5", start, end)
	}
	got.resourceState.LogScroll = 10
	next, _ = got.updateResourceLog(tea.KeyMsg{Type: tea.KeyDown})
	got = next.(Model)
	if got.resourceState.LogScroll != 3 {
		t.Fatalf("scroll should clamp to max, got %d want 3", got.resourceState.LogScroll)
	}
}

func TestResourceLogHeaderShowsNavigationAndRange(t *testing.T) {
	m := Model{
		resourceState: resourceState{
			LogKind: resourceServices,
			LogName: "api.service",
			LogOutput: strings.Join([]string{
				"l1", "l2", "l3", "l4", "l5", "l6",
			}, "\n"),
		},
		appConfig: config.AppConfig{Language: "zh"},
		width:     120,
		height:    8,
		states:    []hostState{{Host: host.Host{Category: "prod", Name: "api-01"}}},
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
		resourceState: resourceState{
			LogKind: resourceContainers,
			LogName: "wireguard",
			LogOutput: strings.Join([]string{
				longLine,
				"short",
				longLine,
				"short",
				longLine,
				"short",
			}, "\n"),
		},
		appConfig: config.AppConfig{Language: "zh"},
		width:     80,
		height:    8,
		states:    []hostState{{Host: host.Host{Category: "aws", Name: "vpn"}}},
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
	m := Model{
		resourceState: resourceState{
			AddKind:    resourceServices,
			ManagePane: 0,
		},
	}
	next, _ := m.updateResourceAdd(tea.KeyMsg{Type: tea.KeyTab})
	got := next.(Model)
	if got.resourceState.AddKind != resourceServices || got.resourceState.ManagePane != 1 {
		t.Fatalf("after tab kind/pane = %v/%d, want services/1", got.resourceState.AddKind, got.resourceState.ManagePane)
	}
	next, _ = got.updateResourceAdd(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	got = next.(Model)
	if got.resourceState.AddKind != resourceProcesses || got.resourceState.ManagePane != 1 {
		t.Fatalf("after g kind/pane = %v/%d, want processes/1", got.resourceState.AddKind, got.resourceState.ManagePane)
	}
	next, _ = got.updateResourceAdd(tea.KeyMsg{Type: tea.KeyLeft})
	got = next.(Model)
	if got.resourceState.AddKind != resourceServices || got.resourceState.ManagePane != 1 {
		t.Fatalf("after left kind/pane = %v/%d, want services/1", got.resourceState.AddKind, got.resourceState.ManagePane)
	}
	next, _ = got.updateResourceAdd(tea.KeyMsg{Type: tea.KeyLeft})
	got = next.(Model)
	if got.resourceState.AddKind != resourceContainers || got.resourceState.ManagePane != 1 {
		t.Fatalf("after second left kind/pane = %v/%d, want containers/1", got.resourceState.AddKind, got.resourceState.ManagePane)
	}
	next, _ = got.updateResourceAdd(tea.KeyMsg{Type: tea.KeyRight})
	got = next.(Model)
	if got.resourceState.AddKind != resourceServices || got.resourceState.ManagePane != 1 {
		t.Fatalf("after right kind/pane = %v/%d, want services/1", got.resourceState.AddKind, got.resourceState.ManagePane)
	}
	next, _ = got.updateResourceAdd(tea.KeyMsg{Type: tea.KeyRight})
	got = next.(Model)
	if got.resourceState.AddKind != resourceProcesses || got.resourceState.ManagePane != 1 {
		t.Fatalf("after second right kind/pane = %v/%d, want processes/1", got.resourceState.AddKind, got.resourceState.ManagePane)
	}
	next, _ = got.updateResourceAdd(tea.KeyMsg{Type: tea.KeyCtrlI})
	got = next.(Model)
	if got.resourceState.AddKind != resourceProcesses || got.resourceState.ManagePane != 0 {
		t.Fatalf("after ctrl+i kind/pane = %v/%d, want processes/0", got.resourceState.AddKind, got.resourceState.ManagePane)
	}
}

func TestResourceManagerHelpMatchesActivePane(t *testing.T) {
	m := Model{
		resourceState: resourceState{
			AddKind:    resourceServices,
			ManagePane: 0,
		}, appConfig: config.AppConfig{Language: "zh"}}
	left := m.resourceManageHelp()
	if !strings.Contains(left, "添加 Enter/f") || strings.Contains(left, "移出") || strings.Contains(left, "编辑 e") {
		t.Fatalf("left pane help = %q, want add-only actions", left)
	}
	m.resourceState.ManagePane = 1
	right := m.resourceManageHelp()
	if !strings.Contains(right, "移出 Enter/x") || !strings.Contains(right, "编辑 e") || strings.Contains(right, "添加") {
		t.Fatalf("right pane help = %q, want remove/edit actions", right)
	}
	m.resourceState.ManagePane = 0
	m.resourceState.AddKind = resourceDatabases
	db := m.resourceManageHelp()
	if !strings.Contains(db, "新建 n") {
		t.Fatalf("database left pane help = %q, want new database shortcut", db)
	}
}

func TestDatabaseEngineFieldCyclesChoicesAndDefaults(t *testing.T) {
	m := Model{
		resourceState: resourceState{
			CommandForm: resourceCommandForm{
				Kind:     resourceDatabases,
				DBEngine: "MySQL",
				DBPort:   "3306",
				DBUser:   "root",
			},
			CommandField: 0,
		},
	}
	m.cycleResourceCommandDatabaseEngine(1)
	if m.resourceState.CommandForm.DBEngine != "MariaDB" || m.resourceState.CommandForm.DBPort != "3306" || m.resourceState.CommandForm.DBUser != "root" {
		t.Fatalf("after mysql->mariadb = %+v", m.resourceState.CommandForm)
	}
	m.cycleResourceCommandDatabaseEngine(1)
	if m.resourceState.CommandForm.DBEngine != "PostgreSQL" || m.resourceState.CommandForm.DBPort != "5432" || m.resourceState.CommandForm.DBUser != "postgres" || m.resourceState.CommandForm.DBName != "postgres" {
		t.Fatalf("after mariadb->postgres = %+v", m.resourceState.CommandForm)
	}
	m.resourceState.CommandForm.DBPort = "35432"
	m.cycleResourceCommandDatabaseEngine(1)
	if m.resourceState.CommandForm.DBEngine != "Redis" || m.resourceState.CommandForm.DBPort != "35432" {
		t.Fatalf("custom port should be preserved, got %+v", m.resourceState.CommandForm)
	}
}

func TestDatabaseDiscoveredAddPrefillsConnectionDefaults(t *testing.T) {
	m := Model{
		resourceState: resourceState{
			HostIndex: 0,
			Kind:      resourceDatabases,
		},
		width: 120,
		states: []hostState{{
			Host: host.Host{Category: "prod", Name: "db-01"},
			DatabaseDetails: []resourceservice.DatabaseDetail{{
				Name:     "postgresql_8fjg-postgresql_8FJG-1",
				Engine:   "PostgreSQL",
				Endpoint: "0.0.0.0:35432->5432/tcp",
				Managed:  true,
			}},
		}},
	}
	next, _ := m.startResourceDatabaseDiscoveredAdd(resourceRef{Kind: resourceDatabases, Index: 0})
	got := next.(Model)
	if got.mode != modeResourceAddEdit ||
		got.resourceState.CommandForm.DBEngine != "PostgreSQL" ||
		got.resourceState.CommandForm.DBHost != "127.0.0.1" ||
		got.resourceState.CommandForm.DBPort != "35432" ||
		got.resourceState.CommandForm.DBUser != "postgres" ||
		got.resourceState.CommandForm.DBName != "postgres" ||
		got.resourceState.CommandForm.DBInstance != "postgresql_8fjg-postgresql_8FJG-1" {
		t.Fatalf("database form = %+v, want discovered postgres defaults", got.resourceState.CommandForm)
	}
}

func TestResourceCommandEditSavesCustomCommand(t *testing.T) {
	home := t.TempDir()
	if err := config.SaveResources(home, config.ResourcesFile{Items: []config.ManagedResource{{
		Server:         "prod/api-01",
		Kind:           config.ResourceKindService,
		Name:           "api.service",
		Added:          true,
		RestartCommand: "systemctl restart api.service",
	}}}); err != nil {
		t.Fatal(err)
	}
	file, _, err := config.LoadResources(home)
	if err != nil {
		t.Fatal(err)
	}
	m := Model{
		resourceState: resourceState{
			HostIndex: 0,
			Kind:      resourceServices,
			Scope:     resourceScopeDiscovered,
			Filter:    resourceFilterAll,
			File:      file,
		},
		home:  home,
		width: 120,
		states: []hostState{{
			Host:           host.Host{Category: "prod", Name: "api-01"},
			ServiceDetails: []resourceservice.ServiceDetail{{Unit: "api.service", Load: "loaded", Active: "active", Sub: "running", Managed: true}},
		}},
	}
	next, _ := m.startResourceCommandEdit()
	got := next.(Model)
	if got.mode != modeResourceCommandEdit {
		t.Fatalf("mode = %v, want command edit", got.mode)
	}
	got.resourceState.CommandField = 2
	got.resourceState.CommandCursor = len([]rune(got.resourceState.CommandForm.RestartCommand))
	got.resourceState.CommandForm.RestartCommand = "cd /data/api && ./restart.sh"
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
		resourceState: resourceState{
			HostIndex: 0,
			Kind:      resourceServices,
		},
		home:   home,
		width:  120,
		states: []hostState{{Host: host.Host{Category: "prod", Name: "api-01"}}},
	}
	next, _ := m.startResourceAdd()
	got := next.(Model)
	got.resourceState.AddName = "api.service"
	got.applyResourceAddDefaults()
	if got.resourceState.CommandForm.RestartCommand != "systemctl restart api.service" {
		t.Fatalf("restart default = %q", got.resourceState.CommandForm.RestartCommand)
	}
	got.resourceState.CommandForm.RestartCommand = "cd /data/api && ./restart.sh"
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

func TestResourceManagerAddsExternalDatabase(t *testing.T) {
	home := t.TempDir()
	m := Model{
		resourceState: resourceState{
			HostIndex: 0,
			Kind:      resourceDatabases,
		},
		home:   home,
		width:  120,
		states: []hostState{{Host: host.Host{Category: "prod", Name: "app-01", JumpHostRef: "bastion"}}},
	}
	next, _ := m.startResourceAdd()
	got := next.(Model)
	got.resourceState.AddKind = resourceDatabases
	next, _ = got.updateResourceAdd(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	got = next.(Model)
	if got.mode != modeResourceAddEdit || got.resourceState.CommandForm.DBEngine != "MySQL" || got.resourceState.CommandForm.DBHost != "127.0.0.1" {
		t.Fatalf("database add mode/defaults = %v/%#v", got.mode, got.resourceState.CommandForm)
	}
	got.resourceState.CommandForm.DBEngine = "PostgreSQL"
	got.resourceState.CommandForm.DBHost = "prod-rds.example.com"
	got.resourceState.CommandForm.DBPort = "5432"
	got.resourceState.CommandForm.DBUser = "monitor"
	got.resourceState.CommandForm.DBPassword = "secret"
	got.resourceState.CommandForm.DBName = "app"
	got.resourceState.CommandForm.DBNote = "核心交易库"
	next, _ = got.saveResourceAdd()
	got = next.(Model)
	if got.mode != modeResourceAdd || got.resourceState.ManagePane != 1 {
		t.Fatalf("mode/pane = %v/%d, want resource manager added pane", got.mode, got.resourceState.ManagePane)
	}
	file, _, err := config.LoadResources(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Items) != 1 {
		t.Fatalf("resources count = %d, want 1: %#v", len(file.Items), file.Items)
	}
	item := file.Items[0]
	if !item.Added || item.Kind != config.ResourceKindDatabase || item.Name != "app" ||
		item.DBEngine != "PostgreSQL" || item.DBHost != "prod-rds.example.com" ||
		item.DBPort != "5432" || item.DBUser != "monitor" || item.DBPassword != "secret" || item.DBName != "app" || item.DBNote != "核心交易库" {
		t.Fatalf("saved database resource = %#v", item)
	}
}

func TestExternalDatabaseResourceIsNotMarkedMissing(t *testing.T) {
	m := Model{
		resourceState: resourceState{
			HostIndex: 0,
			File: config.ResourcesFile{Items: []config.ManagedResource{{
				Server:     "prod/app-01",
				Kind:       config.ResourceKindDatabase,
				Name:       "app",
				Added:      true,
				DBEngine:   "PostgreSQL",
				DBHost:     "prod-rds.example.com",
				DBPort:     "5432",
				DBUser:     "monitor",
				DBName:     "app",
				DBInstance: "",
			}}},
		},
		appConfig: config.AppConfig{Language: "zh"},
		states:    []hostState{{Host: host.Host{Category: "prod", Name: "app-01"}}},
	}
	m.applyManagedResources(0)
	if len(m.states[0].DatabaseDetails) != 1 {
		t.Fatalf("database details count = %d, want 1", len(m.states[0].DatabaseDetails))
	}
	db := m.states[0].DatabaseDetails[0]
	if db.Missing || !db.Managed || !db.Configured {
		t.Fatalf("database detail = %+v, want configured external database not missing", db)
	}
}

func TestExternalDatabaseDetailShowsExternalFoundStateAndConnectedStatus(t *testing.T) {
	m := Model{
		resourceState: resourceState{
			HostIndex: 0,
			Kind:      resourceDatabases,
			Filter:    resourceFilterAll,
			File: config.ResourcesFile{Items: []config.ManagedResource{{
				Server:   "prod/app-01",
				Kind:     config.ResourceKindDatabase,
				Name:     "app",
				Added:    true,
				DBEngine: "PostgreSQL",
				DBHost:   "prod-rds.example.com",
				DBPort:   "5432",
				DBName:   "app",
			}}},
			DatabaseExtraCache: map[string]databaseExtraCache{
				"app": {Detail: resourceservice.DatabaseExtraDetail{Version: "PostgreSQL 16", Raw: map[string]string{"Uptime": "7200"}}},
			},
		},
		appConfig: config.AppConfig{Language: "zh"},
		states:    []hostState{{Host: host.Host{Category: "prod", Name: "app-01"}}},
	}
	m.applyManagedResources(0)
	lines := ansi.Strip(strings.Join(m.resourceDetailLines(), "\n"))
	if !strings.Contains(lines, "发现") || !strings.Contains(lines, "外部") {
		t.Fatalf("database detail lines =\n%s\nwant external found state", lines)
	}
	if !strings.Contains(lines, "运行") || !strings.Contains(lines, "已连接") {
		t.Fatalf("database detail lines =\n%s\nwant connected status", lines)
	}
}

func TestResourceDatabaseConfigTextFieldsAcceptShortcutLetters(t *testing.T) {
	m := Model{
		resourceState: resourceState{
			AddKind:  resourceDatabases,
			AddField: 5,
			CommandForm: resourceCommandForm{
				Kind:     resourceDatabases,
				DBEngine: "PostgreSQL",
				DBName:   "mar",
			},
			AddCursor: 3,
		},
		mode: modeResourceAddEdit,
	}
	next, _ := m.updateResourceAddEdit(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	got := next.(Model)
	if got.resourceState.CommandForm.DBName != "mark" || got.resourceState.AddField != 5 {
		t.Fatalf("DBName/field = %q/%d, want mark/5", got.resourceState.CommandForm.DBName, got.resourceState.AddField)
	}
	next, _ = got.updateResourceAddEdit(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	got = next.(Model)
	if got.resourceState.CommandForm.DBName != "markq" || got.mode != modeResourceAddEdit {
		t.Fatalf("DBName/mode = %q/%v, want markq/add-edit", got.resourceState.CommandForm.DBName, got.mode)
	}
}

func TestResourceDatabaseConfigNoteField(t *testing.T) {
	m := Model{
		resourceState: resourceState{
			AddKind:  resourceDatabases,
			AddField: 6,
			CommandForm: resourceCommandForm{
				Kind:     resourceDatabases,
				DBEngine: "PostgreSQL",
				DBNote:   "tra",
			},
			AddCursor: 3,
		},
		mode: modeResourceAddEdit,
	}
	next, _ := m.updateResourceAddEdit(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	got := next.(Model)
	if got.resourceState.CommandForm.DBNote != "trad" {
		t.Fatalf("DBNote = %q, want trad", got.resourceState.CommandForm.DBNote)
	}
	if resourceCommandFieldCount(resourceDatabases) != 7 {
		t.Fatalf("database field count = %d, want 7", resourceCommandFieldCount(resourceDatabases))
	}
}

func TestCommandEditTextFieldsAcceptShortcutLetters(t *testing.T) {
	m := Model{
		mode:          modeCommandEdit,
		commandField:  1,
		commandForm:   commandEditForm{Name: "bac"},
		commandCursor: 3,
	}
	next, _ := m.updateCommandEdit(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	got := next.(Model)
	if got.commandForm.Name != "back" {
		t.Fatalf("Name = %q, want back", got.commandForm.Name)
	}
	next, _ = got.updateCommandEdit(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	got = next.(Model)
	if got.commandForm.Name != "backq" || got.mode != modeCommandEdit {
		t.Fatalf("Name/mode = %q/%v, want backq/command-edit", got.commandForm.Name, got.mode)
	}
}

func TestDeploymentEditTextFieldsAcceptShortcutLetters(t *testing.T) {
	m := Model{
		mode:             modeDeploymentEdit,
		deploymentField:  3,
		deploymentForm:   deploymentForm{Name: "mar"},
		deploymentCursor: 3,
	}
	next, _ := m.updateDeploymentEdit(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	got := next.(Model)
	if got.deploymentForm.Name != "mark" {
		t.Fatalf("Name = %q, want mark", got.deploymentForm.Name)
	}
	next, _ = got.updateDeploymentEdit(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	got = next.(Model)
	if got.deploymentForm.Name != "markq" || got.mode != modeDeploymentEdit {
		t.Fatalf("Name/mode = %q/%v, want markq/deployment-edit", got.deploymentForm.Name, got.mode)
	}
}

func TestResourceScopedLoadOnlyUpdatesRequestedKind(t *testing.T) {
	oldServiceAt := time.Now().Add(-time.Hour)
	m := Model{
		resourceState: resourceState{
			Kind:        resourceContainers,
			ServiceAt:   oldServiceAt,
			ContainerAt: time.Time{},
		},
		states: []hostState{{
			ServiceDetails:   []resourceservice.ServiceDetail{{Unit: "nginx.service", Load: "loaded", Active: "active", Sub: "running"}},
			ContainerDetails: []resourceservice.ContainerDetail{{Name: "old", Status: "Exited (0)"}},
		}},
	}
	next, _ := m.handleResourceLoad(resourceLoadMsg{
		Index:      0,
		Kind:       resourceContainers,
		Containers: []resourceservice.ContainerDetail{{Name: "api", Status: "Up 1 second"}},
	})
	got := next.(Model)
	if got.states[0].ContainerDetails[0].Name != "api" {
		t.Fatalf("container not updated: %+v", got.states[0].ContainerDetails)
	}
	if got.states[0].ServiceDetails[0].Unit != "nginx.service" {
		t.Fatalf("service should be preserved: %+v", got.states[0].ServiceDetails)
	}
	if !got.resourceState.ServiceAt.Equal(oldServiceAt) || got.resourceState.ContainerAt.IsZero() {
		t.Fatalf("timestamps service=%v container=%v", got.resourceState.ServiceAt, got.resourceState.ContainerAt)
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
	if !got.resourceState.Loading {
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
		Containers: []resourceservice.ContainerDetail{{Name: "api", Image: "app:latest", Status: "Up 1 second", CPU: "0.1%"}},
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
		resourceState: resourceState{
			Kind: resourceServices,
		},
		states: []hostState{{
			ServiceDetails: []resourceservice.ServiceDetail{{Unit: "old.service"}},
			PortDetails:    []resourceservice.PortDetail{{Protocol: "tcp", Port: "22", PID: "1"}},
		}},
	}
	next, _ := m.handleResourceLoad(resourceLoadMsg{
		Index:    0,
		Kind:     resourceServices,
		Services: []resourceservice.ServiceDetail{{Unit: "nginx.service", Load: "loaded", Active: "active", Sub: "running", MainPID: "100"}},
		Ports:    []resourceservice.PortDetail{{Protocol: "tcp", Port: "80", PID: "100"}},
	})
	got := next.(Model)
	if got.states[0].ServiceDetails[0].Unit != "nginx.service" {
		t.Fatalf("service not updated: %+v", got.states[0].ServiceDetails)
	}
	if len(got.states[0].PortDetails) != 1 || got.states[0].PortDetails[0].Port != "80" {
		t.Fatalf("ports should update with service load: %+v", got.states[0].PortDetails)
	}
	if got.resourceState.PortAt.IsZero() {
		t.Fatal("port timestamp should update with service load")
	}
}

func TestResourceTabSwitchesTypeAndGSwitchesStatus(t *testing.T) {
	m := Model{
		resourceState: resourceState{
			Kind:   resourceAll,
			Filter: resourceFilterAll,
		},
	}
	next, _ := m.updateResourceList(tea.KeyMsg{Type: tea.KeyTab})
	got := next.(Model)
	if got.resourceState.Kind != resourceContainers || got.resourceState.Filter != resourceFilterAll {
		t.Fatalf("after tab kind/filter = %v/%v, want containers/all", got.resourceState.Kind, got.resourceState.Filter)
	}
	next, _ = got.updateResourceList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	got = next.(Model)
	if got.resourceState.Filter != resourceFilterRunning || got.resourceState.Kind != resourceContainers {
		t.Fatalf("after g kind/filter = %v/%v, want containers/running", got.resourceState.Kind, got.resourceState.Filter)
	}
}

func TestResourcePortGSwitchesScopeFilter(t *testing.T) {
	m := Model{
		resourceState: resourceState{
			HostIndex:  0,
			Kind:       resourcePorts,
			Filter:     resourceFilterStopped,
			PortFilter: resourcePortFilterAll,
		},
		width: 120,
		states: []hostState{{
			PortDetails: []resourceservice.PortDetail{
				{Protocol: "tcp", Port: "22", LocalAddress: "0.0.0.0:22", Process: "sshd", PID: "1", Managed: true},
				{Protocol: "tcp", Port: "25", LocalAddress: "127.0.0.1:25", Process: "master", PID: "2", Managed: true},
				{Protocol: "tcp", Port: "3306", LocalAddress: "172.31.1.10:3306", Process: "mysqld", PID: "3", Managed: true},
				{Protocol: "tcp", Port: "1080", LocalAddress: "0.0.0.0:1080", Process: "docker-proxy", PID: "4", Container: "socks5", Managed: true},
			},
		}},
	}
	next, _ := m.updateResourceList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	got := next.(Model)
	if got.resourceState.PortFilter != resourcePortFilterPublic || got.resourceState.Filter != resourceFilterStopped {
		t.Fatalf("filters after g = port %v resource %v, want public and unchanged status", got.resourceState.PortFilter, got.resourceState.Filter)
	}
	indexes := got.filteredResourceIndexes()
	want := []resourceRef{{Kind: resourcePorts, Index: 0}, {Kind: resourcePorts, Index: 3}}
	if !reflect.DeepEqual(indexes, want) {
		t.Fatalf("public port indexes = %#v, want %#v", indexes, want)
	}
	got.resourceState.PortFilter = resourcePortFilterLoopback
	indexes = got.filteredResourceIndexes()
	want = []resourceRef{{Kind: resourcePorts, Index: 1}}
	if !reflect.DeepEqual(indexes, want) {
		t.Fatalf("loopback port indexes = %#v, want %#v", indexes, want)
	}
	got.resourceState.PortFilter = resourcePortFilterSpecific
	indexes = got.filteredResourceIndexes()
	want = []resourceRef{{Kind: resourcePorts, Index: 2}}
	if !reflect.DeepEqual(indexes, want) {
		t.Fatalf("specific port indexes = %#v, want %#v", indexes, want)
	}
	got.resourceState.PortFilter = resourcePortFilterContainer
	indexes = got.filteredResourceIndexes()
	want = []resourceRef{{Kind: resourcePorts, Index: 3}}
	if !reflect.DeepEqual(indexes, want) {
		t.Fatalf("container port indexes = %#v, want %#v", indexes, want)
	}
}

func TestResourceManagerPortsDeduplicatesProtocolPort(t *testing.T) {
	m := Model{
		resourceState: resourceState{
			HostIndex: 0,
			AddKind:   resourcePorts,
			Filter:    resourceFilterAll,
		},
		width: 120,
		states: []hostState{{
			PortDetails: []resourceservice.PortDetail{
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
		resourceState: resourceState{
			HostIndex: 0,
			Kind:      resourceContainers,
			Index:     0,
		},
		width: 120,
		states: []hostState{{
			ContainerDetails: []resourceservice.ContainerDetail{{Name: "api", Status: "running", Managed: true}},
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
		resourceState: resourceState{
			HostIndex: 0,
			Kind:      resourceProcesses,
			Scope:     resourceScopeDiscovered,
			Filter:    resourceFilterAll,
			File: config.ResourcesFile{Items: []config.ManagedResource{{
				Server:         "prod/api-01",
				Kind:           config.ResourceKindProcess,
				Name:           "go-api",
				Added:          true,
				StartCommand:   "systemctl start go-api",
				StopCommand:    "systemctl stop go-api",
				RestartCommand: "systemctl restart go-api",
				LogCommand:     "journalctl -u go-api -n 200 --no-pager",
			}}},
		},
		width: 120,
		states: []hostState{{
			Host:        host.Host{Category: "prod", Name: "api-01"},
			PortDetails: []resourceservice.PortDetail{{Protocol: "tcp", Port: "8080", LocalAddress: "0.0.0.0:8080", Process: "go-api", PID: "200", ProcessManaged: true}},
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
	if got.mode != modeResourceConfirm || got.resourceState.ActionResource != resourceProcesses {
		t.Fatalf("mode/action = %v/%v, want process confirm", got.mode, got.resourceState.ActionResource)
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
		resourceState: resourceState{
			HostIndex: 0,
			Kind:      resourceProcesses,
			Scope:     resourceScopeDiscovered,
			File: config.ResourcesFile{Items: []config.ManagedResource{{
				Server: "prod/api-01",
				Kind:   config.ResourceKindProcess,
				Name:   "go-api",
				Added:  true,
			}}},
			Filter: resourceFilterAll,
		},
		width: 120,
		states: []hostState{{
			Host:        host.Host{Category: "prod", Name: "api-01"},
			PortDetails: []resourceservice.PortDetail{{Protocol: "tcp", Port: "8080", LocalAddress: "0.0.0.0:8080", Process: "go-api", PID: "200", ProcessManaged: true}},
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
		resourceState: resourceState{
			HostIndex: 0,
			Kind:      resourceProcesses,
			Scope:     resourceScopeDiscovered,
			Filter:    resourceFilterAll,
		},
		width: 120,
		states: []hostState{{
			PortDetails: []resourceservice.PortDetail{{Protocol: "tcp", Port: "8080", LocalAddress: "0.0.0.0:8080", Process: "go-api", PID: "200"}},
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

func TestResourceCardMovementUpdatesSelection(t *testing.T) {
	m := Model{
		resourceState: resourceState{
			HostIndex: 0,
			Kind:      resourceContainers,
			View:      resourceViewCards,
			Index:     0,
		},
		width: 120,
		states: []hostState{{
			ContainerDetails: []resourceservice.ContainerDetail{
				{Name: "one", Status: "Up 1 second", Managed: true},
				{Name: "two", Status: "Up 1 second", Managed: true},
				{Name: "three", Status: "Up 1 second", Managed: true},
				{Name: "four", Status: "Up 1 second", Managed: true},
			},
		}},
	}
	m.moveResourceRight()
	if m.resourceState.Index != 1 {
		t.Fatalf("right resourceIndex = %d, want 1", m.resourceState.Index)
	}
	m.moveResourceLeft()
	if m.resourceState.Index != 0 {
		t.Fatalf("left resourceIndex = %d, want 0", m.resourceState.Index)
	}
	m.moveResourceDown()
	if m.resourceState.Index != 3 {
		t.Fatalf("down resourceIndex = %d, want 3", m.resourceState.Index)
	}
	m.moveResourceDown()
	if m.resourceState.Index != 3 {
		t.Fatalf("down at bottom resourceIndex = %d, want clamp at 3", m.resourceState.Index)
	}
}

func TestDeploymentRollbackConfirmRunsOnlyRollbackFlow(t *testing.T) {
	app := config.DeploymentApp{
		Name:             "api",
		Server:           "prod/api",
		Source:           config.DeploySourceGit,
		Path:             "/srv/api",
		ResourceCommands: []string{"git pull"},
		UpdateCommands:   []string{"make deploy"},
		HealthCommands:   []string{"curl -f localhost"},
		RollbackCommands: []string{"ln -sfn releases/old current", "systemctl restart api"},
	}
	m := Model{
		width:  100,
		height: 24,
		activeDeployment: activeDeployment{
			App:             app,
			Action:          config.DeployActionDeploy,
			PreviousVersion: "old",
			CurrentVersion:  "new",
		},
	}
	view := m.renderDeploymentRollbackConfirm()
	for _, want := range []string{"Confirm Rollback", "Previous version", "old", "Current version", "new", "Rollback commands", "ln -sfn releases/old current"} {
		if !strings.Contains(view, want) {
			t.Fatalf("rollback confirm missing %q:\n%s", want, view)
		}
	}
	for _, notWant := range []string{"git pull", "make deploy", "curl -f localhost", "Fetch", "Health"} {
		if strings.Contains(view, notWant) {
			t.Fatalf("rollback confirm leaked deploy flow %q:\n%s", notWant, view)
		}
	}
	next, cmd := m.updateDeploymentRollbackConfirm(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(Model)
	if got.mode != modeDeploymentOutput || got.activeDeployment.Action != config.DeployActionRollback || !got.activeDeployment.Running || got.activeDeployment.ProgressID == "" || cmd == nil {
		t.Fatalf("rollback enter state = mode %v active %+v cmd %v", got.mode, got.activeDeployment, cmd)
	}
}

func TestTransferStatusFilterKeepsSelectionVisible(t *testing.T) {
	m := Model{
		mode:   modeTransferJobs,
		width:  120,
		height: 30,
		transferState: transferState{
			Index: 1,
			History: config.TransferHistoryFile{Entries: []config.TransferEntry{
				{ID: "queued-1", Status: config.TransferStatusQueued, Source: "/tmp/a", TargetDir: "/srv"},
				{ID: "done-1", Status: config.TransferStatusDone, Source: "/tmp/b", TargetDir: "/srv"},
				{ID: "failed-1", Status: config.TransferStatusFailed, Source: "/tmp/c", TargetDir: "/srv"},
			}},
		},
	}
	m.cycleTransferStatusFilter()
	if got := m.transferStatusFilterValue(); got != config.TransferStatusQueued {
		t.Fatalf("filter = %q, want queued", got)
	}
	if m.transferState.Index != 0 {
		t.Fatalf("transferIndex = %d, want first queued entry", m.transferState.Index)
	}
	m.cycleTransferStatusFilter()
	if got := m.transferStatusFilterValue(); got != config.TransferStatusPending {
		t.Fatalf("filter = %q, want pending", got)
	}
	if m.transferState.Index != 0 {
		t.Fatalf("empty filtered transferIndex = %d, want stable fallback 0", m.transferState.Index)
	}
	view := ansi.Strip(m.renderTransferJobs())
	if !strings.Contains(view, "No transfer jobs for this status") && !strings.Contains(view, "当前状态没有传输任务") {
		t.Fatalf("empty status filter view missing empty message:\n%s", view)
	}
}

func TestBatchCommandDoneAdvancesAndSummarizesResults(t *testing.T) {
	m := Model{
		home:             t.TempDir(),
		mode:             modeBatchOutput,
		batchCommand:     commandItem{Name: "uptime", Command: "uptime"},
		batchCurrent:     0,
		batchOutputIndex: 0,
		batchJobs: []batchJob{
			{HostIndex: 0, Running: true},
			{HostIndex: 1},
		},
		states: []hostState{
			{Host: host.Host{Name: "api", Category: "prod"}},
			{Host: host.Host{Name: "web", Category: "prod"}},
		},
	}
	next, cmd := m.handleBatchCommandDone(batchCommandDoneMsg{Job: 0, Result: commandResult{Output: "ok", ExitCode: 0}})
	got := next.(Model)
	if cmd == nil || got.batchCurrent != 1 || !got.batchJobs[0].Done || got.batchJobs[0].Running || !got.batchJobs[1].Running || got.batchOutputIndex != 1 {
		t.Fatalf("after first batch result active=%+v cmd=%v", got, cmd)
	}
	next, cmd = got.handleBatchCommandDone(batchCommandDoneMsg{Job: 1, Result: commandResult{Err: errors.New("denied"), ExitCode: 255, Output: "permission denied"}})
	got = next.(Model)
	if cmd != nil {
		t.Fatalf("final batch result scheduled unexpected command")
	}
	if got.batchCurrent != 2 || got.batchJobs[1].Running || !got.batchJobs[1].Done || got.batchSuccessCount() != 1 || got.batchFailCount() != 1 {
		t.Fatalf("after final batch result active=%+v", got)
	}
	if !strings.Contains(got.status, "成功1") || !strings.Contains(got.status, "失败1") {
		t.Fatalf("batch completion status = %q", got.status)
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
