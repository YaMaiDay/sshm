package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func (m *Model) move(delta int) {
	count := len(m.filteredIndexes())
	if count == 0 {
		m.selected = 0
		return
	}
	m.selected += delta
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= count {
		m.selected = count - 1
	}
}

func (m *Model) moveDashboardDown() {
	if m.dashboard.Mode == dashboardCategory && m.dashboard.Focus == 0 {
		m.moveDashboardCategory(1)
		return
	}
	if m.dashboard.Mode == dashboardCategory {
		m.move(1)
		return
	}
	if m.dashboard.Mode == dashboardCards {
		m.move(m.dashboardColumns())
		return
	}
	m.move(1)
}

func (m *Model) moveDashboardUp() {
	if m.dashboard.Mode == dashboardCategory && m.dashboard.Focus == 0 {
		m.moveDashboardCategory(-1)
		return
	}
	if m.dashboard.Mode == dashboardCategory {
		m.move(-1)
		return
	}
	if m.dashboard.Mode == dashboardCards {
		m.move(-m.dashboardColumns())
		return
	}
	m.move(-1)
}

func (m *Model) moveDashboardLeft() {
	if m.dashboard.Mode == dashboardCategory {
		m.dashboard.Focus = 0
		return
	}
	m.move(-1)
}

func (m *Model) moveDashboardRight() {
	if m.dashboard.Mode == dashboardCategory {
		if m.dashboard.Focus == 0 {
			m.dashboard.Focus = 1
		}
		return
	}
	m.move(1)
}

func (m *Model) moveDashboardCategory(delta int) {
	items := m.dashboardCategoryItems()
	if len(items) == 0 {
		return
	}
	index := m.dashboardCategorySelectedIndex(items)
	index = clampInt(index+delta, 0, len(items)-1)
	m.applyDashboardCategoryItem(items[index])
}

func clampInt(value int, min int, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func moveClampedInt(current int, delta int, min int, max int) int {
	current = clampInt(current, min, max)
	return clampInt(current+delta, min, max)
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func fitLines(lines []string, width int) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, ansi.Truncate(line, width, "…"))
	}
	return out
}

func visibleRange(total int, selected int, height int) (int, int) {
	if total <= 0 {
		return 0, 0
	}
	if height <= 0 || height >= total {
		return 0, total
	}
	selected = clampInt(selected, 0, total-1)
	start := selected - height + 1
	if start < 0 {
		start = 0
	}
	if start+height > total {
		start = total - height
	}
	return start, start + height
}

func (m Model) selectedRealIndex() (int, bool) {
	indexes := m.filteredIndexes()
	if len(indexes) == 0 || m.selected < 0 || m.selected >= len(indexes) {
		return 0, false
	}
	return indexes[m.selected], true
}

func (m Model) filteredIndexes() []int {
	var indexes []int
	q := strings.ToLower(strings.TrimSpace(m.query))
	for i, state := range m.states {
		h := state.Host
		if m.category != "" && h.Category != m.category {
			continue
		}
		if m.favoriteOnly && !h.Favorite {
			continue
		}
		if m.filter == filterOnline && !state.Metrics.Online {
			continue
		}
		if m.filter == filterProblem && !m.isProblem(state) {
			continue
		}
		text := strings.ToLower(strings.Join([]string{
			h.Name, h.HostName, h.User, h.Category, h.Note, h.ExpireAt,
		}, " "))
		if q == "" || strings.Contains(text, q) {
			indexes = append(indexes, i)
		}
	}
	sort.SliceStable(indexes, func(i, j int) bool {
		a := m.states[indexes[i]]
		b := m.states[indexes[j]]
		if m.sortCategoryBeforePinned() && a.Host.Category != b.Host.Category {
			return a.Host.Category < b.Host.Category
		}
		if a.Host.Pinned != b.Host.Pinned {
			return a.Host.Pinned
		}
		if a.Host.Pinned && b.Host.Pinned && a.Host.PinnedOrder != b.Host.PinnedOrder {
			return a.Host.PinnedOrder > b.Host.PinnedOrder
		}
		switch m.sortBy {
		case sortState:
			if a.Metrics.Online != b.Metrics.Online {
				return a.Metrics.Online
			}
		case sortCPU:
			return a.Metrics.CPUPercent > b.Metrics.CPUPercent
		case sortMem:
			return a.Metrics.MemPercent() > b.Metrics.MemPercent()
		case sortDisk:
			return a.Metrics.DiskPercent() > b.Metrics.DiskPercent()
		}
		if a.Host.Category == b.Host.Category {
			return a.Host.Name < b.Host.Name
		}
		return a.Host.Category < b.Host.Category
	})
	return indexes
}

func (m Model) sortCategoryBeforePinned() bool {
	return m.dashboard.Mode == dashboardGrouped || m.category != ""
}

func (m Model) isProblem(state hostState) bool {
	if !state.Metrics.Online && !state.Loading {
		return true
	}
	thresholds := m.metricThresholds()
	return state.Metrics.CPUPercent >= thresholds.CPUCrit || state.Metrics.MemPercent() >= thresholds.MemCrit || state.Metrics.DiskPercent() >= thresholds.DiskCrit || state.Metrics.FailedServices > 0
}

func (m Model) sortName() string {
	switch m.sortBy {
	case sortState:
		return m.t("Status", "状态")
	case sortCPU:
		return "CPU"
	case sortMem:
		return m.t("Memory", "内存")
	case sortDisk:
		return m.t("Disk", "磁盘")
	default:
		return m.t("Default", "默认")
	}
}

func (m Model) dashboardHeaderText(indexes []int) string {
	parts := []string{
		titleStyle.Render("sshm"),
		mutedStyle.Render(m.dashboardServerCountText(len(indexes))),
		blueStyle.Render(m.dashboardModeName(m.dashboard.Mode)),
		m.dashboardCategoryHeaderText(),
	}
	if m.searching {
		searchWidth := m.width / 3
		if searchWidth < 8 {
			searchWidth = 8
		}
		parts = append(parts, blueStyle.Render(m.t("Search ", "搜索 ")+inlineCursorText(m.query, searchWidth, len([]rune(m.query)))))
	} else if m.query != "" {
		parts = append(parts, blueStyle.Render(m.t("Search ", "搜索 ")+m.query))
	}
	if m.filter != filterAll {
		parts = append(parts, m.dashboardFilterHeaderText())
	}
	if m.favoriteOnly {
		parts = append(parts, favoriteStyle.Render(m.t("Favorites", "收藏")))
	}
	if m.sortBy != sortDefault {
		parts = append(parts, mutedStyle.Render(m.sortName()))
	}
	if refresh := m.dashboardRefreshHeaderText(); refresh != "" {
		parts = append(parts, mutedStyle.Render(refresh))
	}
	if m.status != "" && m.status != m.refreshStatus {
		parts = append(parts, m.dashboardStatusHeaderText(m.status))
	}
	return strings.Join(parts, "  ")
}

func (m Model) dashboardServerCountText(count int) string {
	if m.isChineseUI() {
		return fmt.Sprintf("%d台", count)
	}
	if count == 1 {
		return "1 server"
	}
	return fmt.Sprintf("%d servers", count)
}

func (m Model) dashboardCategoryHeaderText() string {
	category := m.category
	if m.dashboard.Mode == dashboardCategory {
		category = m.dashboardCategoryActiveLabel()
	}
	if strings.TrimSpace(category) == "" {
		category = m.t("All", "全部")
	}
	if category == m.t("All", "全部") {
		return detailValueStyle.Render(category)
	}
	return blueStyle.Render(category)
}

func (m Model) dashboardFilterHeaderText() string {
	name := m.filterName()
	switch m.filter {
	case filterOnline:
		return greenStyle.Render(name)
	case filterProblem:
		return redStyle.Render(name)
	default:
		return detailValueStyle.Render(name)
	}
}

func (m Model) dashboardRefreshHeaderText() string {
	status := strings.TrimSpace(m.refreshStatus)
	if status == "" {
		return ""
	}
	prefixes := []string{
		m.t("Manual refresh done: ", "手动刷新完成："),
		m.t("Last refresh: ", "最后刷新："),
	}
	for _, prefix := range prefixes {
		status = strings.TrimPrefix(status, prefix)
	}
	return status
}

func (m Model) dashboardStatusHeaderText(status string) string {
	text := strings.TrimSuffix(strings.TrimSpace(status), "。")
	lower := strings.ToLower(text)
	if strings.Contains(lower, "fail") || strings.Contains(lower, "error") || strings.Contains(text, "失败") || strings.Contains(text, "错误") {
		return redStyle.Render(text)
	}
	if strings.Contains(lower, "cancel") || strings.Contains(lower, "unfavorite") || strings.Contains(lower, "unpin") || strings.Contains(text, "取消") {
		return yellowStyle.Render(text)
	}
	if strings.Contains(lower, "saved") || strings.Contains(lower, "added") || strings.Contains(lower, "favorited") || strings.Contains(lower, "pinned") ||
		strings.Contains(text, "保存") || strings.Contains(text, "添加") || strings.Contains(text, "收藏") || strings.Contains(text, "置顶") {
		return greenStyle.Render(text)
	}
	return yellowStyle.Render(text)
}

func (m Model) filterName() string {
	switch m.filter {
	case filterOnline:
		return m.t("Online", "在线")
	case filterProblem:
		return m.t("Problems", "异常")
	default:
		return m.t("All", "全部")
	}
}

func (m *Model) cycleCategory() {
	m.favoriteOnly = false
	categories := []string{""}
	seen := map[string]bool{}
	for _, state := range m.states {
		cat := state.Host.Category
		if cat != "" && !seen[cat] {
			seen[cat] = true
			categories = append(categories, cat)
		}
	}
	sort.Strings(categories[1:])
	current := 0
	for i, cat := range categories {
		if cat == m.category {
			current = i
			break
		}
	}
	m.category = categories[(current+1)%len(categories)]
	if m.category == "" {
		m.status = m.t("Category: All", "分类：全部")
	} else {
		m.status = m.t("Category: ", "分类：") + m.category
	}
}
