package tui

import (
	"testing"

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
