package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/YaMaiDay/sshm/internal/fsselect"
	"github.com/YaMaiDay/sshm/internal/host"
)

func (m *Model) startLocalTree(title string, mode viewMode, dirsOnly bool) {
	m.startTree(title, mode, m.localRootItems(dirsOnly), -1, dirsOnly, true)
}

func newTree(roots []fsselect.Item, hostIndex int, dirsOnly bool, local bool) remoteTree {
	tree := remoteTree{
		HostIndex: hostIndex,
		Local:     local,
		DirsOnly:  dirsOnly,
		Nodes:     map[string]*remoteTreeNode{},
	}
	for _, item := range roots {
		if item.Path == "" || (dirsOnly && !item.IsDir) {
			continue
		}
		if _, ok := tree.Nodes[item.Path]; ok {
			continue
		}
		tree.Roots = append(tree.Roots, item.Path)
		tree.Nodes[item.Path] = &remoteTreeNode{Item: item}
	}
	sort.Strings(tree.Roots)
	return tree
}

func (m *Model) startRemoteTree(title string, mode viewMode, h host.Host, dirsOnly bool) {
	m.startTree(title, mode, m.remoteRootItems(h), m.transferState.Pending.HostIndex, dirsOnly, false)
}

func (m *Model) startRemoteTreeAt(title string, mode viewMode, h host.Host, root string, dirsOnly bool) {
	if root == "" {
		m.startRemoteTree(title, mode, h, dirsOnly)
		return
	}
	m.startTree(title, mode, []fsselect.Item{{Path: root, IsDir: true}}, m.transferState.Pending.HostIndex, dirsOnly, false)
	if len(m.transferState.Choices) > 0 {
		_, _ = m.expandTreePick()
	}
}

func (m *Model) startTree(title string, mode viewMode, roots []fsselect.Item, hostIndex int, dirsOnly bool, local bool) {
	tree := newTree(roots, hostIndex, dirsOnly, local)
	m.transferState.RemoteTree = tree
	m.transferState.PickTitle = title
	m.mode = mode
	m.transferState.PickIndex = 0
	m.refreshTreeChoices()
	if len(m.transferState.Choices) == 0 {
		m.status = title + m.t(": no selectable items", "：没有可选择的项目")
	} else {
		m.status = title
	}
}

func (m Model) treePickerActive() bool {
	switch m.mode {
	case modePickLocalItem, modePickRemoteDir, modePickRemoteItem, modePickSaveDir:
		return m.transferState.RemoteTree.Nodes != nil
	default:
		return false
	}
}

func (m *Model) refreshTreeChoices() {
	var choices []choice
	for _, root := range m.transferState.RemoteTree.Roots {
		m.appendTreeChoice(&choices, root)
	}
	m.transferState.Choices = choices
	if m.transferState.PickIndex >= len(m.transferState.Choices) {
		m.transferState.PickIndex = len(m.transferState.Choices) - 1
	}
	if m.transferState.PickIndex < 0 {
		m.transferState.PickIndex = 0
	}
}

func (m *Model) appendTreeChoice(choices *[]choice, path string) {
	node := m.transferState.RemoteTree.Nodes[path]
	if node == nil {
		return
	}
	label := treeLabel(node)
	*choices = append(*choices, choice{Label: label, Value: node.Item.Path, IsDir: node.Item.IsDir, Depth: node.Depth})
	if !node.Expanded {
		return
	}
	for _, child := range node.Children {
		m.appendTreeChoice(choices, child)
	}
}

func flattenTree(tree remoteTree) []choice {
	var choices []choice
	for _, root := range tree.Roots {
		appendTreeChoiceTo(&choices, tree, root)
	}
	return choices
}

func appendTreeChoiceTo(choices *[]choice, tree remoteTree, path string) {
	node := tree.Nodes[path]
	if node == nil {
		return
	}
	*choices = append(*choices, choice{Label: treeLabel(node), Value: node.Item.Path, IsDir: node.Item.IsDir, Depth: node.Depth})
	if !node.Expanded {
		return
	}
	for _, child := range node.Children {
		appendTreeChoiceTo(choices, tree, child)
	}
}

func (m Model) expandTreePick() (tea.Model, tea.Cmd) {
	if len(m.transferState.Choices) == 0 || m.transferState.PickIndex < 0 || m.transferState.PickIndex >= len(m.transferState.Choices) {
		return m, nil
	}
	pick := m.transferState.Choices[m.transferState.PickIndex]
	node := m.transferState.RemoteTree.Nodes[pick.Value]
	if node == nil || !node.Item.IsDir {
		return m, nil
	}
	if !node.Loaded {
		m.loadTreeNode(node)
	}
	if len(node.Children) == 0 {
		if m.transferState.RemoteTree.DirsOnly {
			m.status = m.t("No subdirectories: ", "没有子目录：") + node.Item.Path + m.t(". Press Space to select current directory.", "。按空格可选择当前目录。")
		} else {
			m.status = m.t("Directory is empty or permission denied: ", "目录为空或没有权限：") + node.Item.Path
		}
		return m, nil
	}
	node.Expanded = true
	m.refreshTreeChoices()
	return m, nil
}

func (m Model) toggleTreePick() (tea.Model, tea.Cmd) {
	if len(m.transferState.Choices) == 0 || m.transferState.PickIndex < 0 || m.transferState.PickIndex >= len(m.transferState.Choices) {
		return m, nil
	}
	pick := m.transferState.Choices[m.transferState.PickIndex]
	node := m.transferState.RemoteTree.Nodes[pick.Value]
	if node == nil || !node.Item.IsDir {
		return m.confirmPick()
	}
	if node.Expanded {
		node.Expanded = false
		m.refreshTreeChoices()
		return m, nil
	}
	return m.expandTreePick()
}

func (m Model) collapseTreePick() Model {
	if len(m.transferState.Choices) == 0 || m.transferState.PickIndex < 0 || m.transferState.PickIndex >= len(m.transferState.Choices) {
		return m
	}
	pick := m.transferState.Choices[m.transferState.PickIndex]
	node := m.transferState.RemoteTree.Nodes[pick.Value]
	if node == nil || !node.Item.IsDir {
		return m
	}
	if node.Expanded {
		node.Expanded = false
		m.refreshTreeChoices()
		return m
	}
	parent := filepath.Dir(node.Item.Path)
	for parent != "." && parent != "/" {
		if _, ok := m.transferState.RemoteTree.Nodes[parent]; ok {
			for i, choice := range m.transferState.Choices {
				if choice.Value == parent {
					m.transferState.PickIndex = i
					return m
				}
			}
		}
		parent = filepath.Dir(parent)
	}
	return m
}

func (m *Model) loadTreeNode(node *remoteTreeNode) {
	loadTreeNodeFor(&m.transferState.RemoteTree, node, m.states)
}

func loadTreeNodeFor(tree *remoteTree, node *remoteTreeNode, states []hostState) {
	var items []fsselect.Item
	if tree.Local {
		items = localTreeItems(node.Item.Path, tree.DirsOnly)
	} else if tree.HostIndex >= 0 && tree.HostIndex < len(states) {
		h := states[tree.HostIndex].Host
		if tree.DirsOnly {
			items = fsselect.RemoteDirItems(h, node.Item.Path)
		} else {
			items = fsselect.RemoteItems(h, node.Item.Path)
		}
	}
	node.Children = nil
	seen := map[string]bool{}
	for _, item := range items {
		if item.Path == "" || item.Path == node.Item.Path {
			continue
		}
		if seen[item.Path] {
			continue
		}
		seen[item.Path] = true
		if tree.DirsOnly && !item.IsDir {
			continue
		}
		tree.Nodes[item.Path] = &remoteTreeNode{Item: item, Depth: node.Depth + 1}
		node.Children = append(node.Children, item.Path)
	}
	sort.Slice(node.Children, func(i, j int) bool {
		a := tree.Nodes[node.Children[i]].Item
		b := tree.Nodes[node.Children[j]].Item
		if a.IsDir != b.IsDir {
			return a.IsDir
		}
		return strings.ToLower(filepath.Base(a.Path)) < strings.ToLower(filepath.Base(b.Path))
	})
	node.Loaded = true
}

func localTreeItems(dir string, dirsOnly bool) []fsselect.Item {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	items := make([]fsselect.Item, 0, len(entries))
	seenRealPaths := map[string]bool{}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		isDir := entry.IsDir()
		if !isDir {
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				isDir = true
			}
		}
		if dirsOnly && !isDir {
			continue
		}
		if isDir {
			realPath := localRealPath(path)
			if realPath != "" {
				if seenRealPaths[realPath] {
					continue
				}
				seenRealPaths[realPath] = true
			}
		}
		items = append(items, fsselect.Item{Path: path, IsDir: isDir})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return strings.ToLower(filepath.Base(items[i].Path)) < strings.ToLower(filepath.Base(items[j].Path))
	})
	return items
}

func localRealPath(path string) string {
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(realPath)
}

func (m Model) localRootItems(dirsOnly bool) []fsselect.Item {
	if !m.appConfig.CustomDirs || len(m.appConfig.LocalDirs) == 0 {
		return localTreeItems("/", dirsOnly)
	}
	roots := fsselect.ExpandLocalRoots(m.home, m.transferLocalDirs())
	items := make([]fsselect.Item, 0, len(roots))
	seen := map[string]bool{}
	seenRealPaths := map[string]bool{}
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" || seen[root] {
			continue
		}
		info, err := os.Stat(root)
		if err != nil {
			continue
		}
		if dirsOnly && !info.IsDir() {
			continue
		}
		if info.IsDir() {
			realPath := localRealPath(root)
			if realPath != "" {
				if seenRealPaths[realPath] {
					continue
				}
				seenRealPaths[realPath] = true
			}
		}
		seen[root] = true
		items = append(items, fsselect.Item{Path: root, IsDir: info.IsDir()})
	}
	if len(items) == 0 {
		return localTreeItems("/", dirsOnly)
	}
	sortItemsByPath(items)
	return items
}

func (m Model) remoteRootItems(h host.Host) []fsselect.Item {
	if !m.appConfig.CustomDirs || len(m.appConfig.RemoteDirs) == 0 {
		return fsselect.RemoteRootItems(h)
	}
	return fsselect.RemoteConfiguredRootItems(h, m.transferRemoteDirs())
}

func (m Model) transferLocalDirs() []string {
	return m.appConfig.LocalDirs
}

func (m Model) transferRemoteDirs() []string {
	return m.appConfig.RemoteDirs
}

func sortItemsByPath(items []fsselect.Item) {
	sort.Slice(items, func(i, j int) bool {
		return strings.ToLower(items[i].Path) < strings.ToLower(items[j].Path)
	})
}

func treeLabel(node *remoteTreeNode) string {
	indent := strings.Repeat("  ", node.Depth)
	name := node.Item.Path
	if node.Depth > 0 {
		name = filepath.Base(node.Item.Path)
	}
	return indent + name
}
