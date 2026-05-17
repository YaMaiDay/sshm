package fsselect

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/YaMaiDay/sshm/internal/host"
	"github.com/YaMaiDay/sshm/internal/sshconfig"
)

type Item struct {
	Path  string
	IsDir bool
}

func LocalRoots(home string) []string {
	return ExpandLocalRoots(home, []string{
		".",
		"~/Downloads",
		"~/Desktop",
		"~/Documents",
		"~",
	})
}

func RemoteRoots() []string {
	return []string{"$HOME", "/home", "/opt", "/var/www", "/www", "/data", "/tmp"}
}

func RemoteRootItems(h host.Host) []Item {
	script := `find / -maxdepth 1 -type d ! -path / 2>/dev/null | while IFS= read -r p; do printf "D	%s\n" "$p"; done`
	out, err := runSSH(h, script)
	if err != nil && strings.TrimSpace(out) == "" {
		return nil
	}
	return parseRemoteItems(out)
}

func RemoteConfiguredRootItems(h host.Host, roots []string) []Item {
	if len(roots) == 0 {
		return RemoteRootItems(h)
	}
	var b strings.Builder
	b.WriteString("for d in")
	count := 0
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		count++
		b.WriteByte(' ')
		b.WriteString(shellQuote(root))
	}
	if count == 0 {
		return RemoteRootItems(h)
	}
	b.WriteString(`; do case "$d" in '$HOME') d="$HOME" ;; '~') d="$HOME" ;; '~/'*) d="$HOME/${d#~/}" ;; esac; [ -d "$d" ] && printf "D	%s\n" "$d"; done`)
	out, err := runSSH(h, b.String())
	if err != nil && strings.TrimSpace(out) == "" {
		return RemoteRootItems(h)
	}
	items := parseRemoteItems(out)
	if len(items) == 0 {
		return RemoteRootItems(h)
	}
	return items
}

func ExpandLocalRoots(home string, roots []string) []string {
	if len(roots) == 0 {
		return LocalRoots(home)
	}
	out := make([]string, 0, len(roots))
	for _, root := range roots {
		if root == "~" {
			out = append(out, home)
		} else if strings.HasPrefix(root, "~/") {
			out = append(out, filepath.Join(home, strings.TrimPrefix(root, "~/")))
		} else {
			out = append(out, root)
		}
	}
	return out
}

func LocalItems(root string) []Item {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	items := make([]Item, 0, len(entries))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		items = append(items, Item{Path: filepath.Join(root, entry.Name()), IsDir: entry.IsDir()})
	}
	sortItems(items)
	return items
}

func RemoteDirs(h host.Host) []string {
	script := `for d in "$HOME" /home /opt /var/www /www /data /tmp; do [ -d "$d" ] && printf "%s\n" "$d"; done; find "$HOME" /home /opt /var/www /www /data /tmp -maxdepth 3 -type d 2>/dev/null`
	out, err := runSSH(h, script)
	if err != nil && strings.TrimSpace(out) == "" {
		return nil
	}
	seen := map[string]bool{}
	var dirs []string
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !seen[line] {
			seen[line] = true
			dirs = append(dirs, line)
		}
	}
	sort.Strings(dirs)
	return dirs
}

func RemoteItems(h host.Host, dir string) []Item {
	quoted := shellQuote(dir)
	script := fmt.Sprintf(`find %s -maxdepth 1 \( -type f -o -type d \) ! -path %s 2>/dev/null | while IFS= read -r p; do if [ -d "$p" ]; then printf "D	%%s\n" "$p"; else printf "F	%%s\n" "$p"; fi; done`, quoted, quoted)
	out, err := runSSH(h, script)
	if err != nil && strings.TrimSpace(out) == "" {
		return nil
	}
	return parseRemoteItems(out)
}

func RemoteDirItems(h host.Host, dir string) []Item {
	items := RemoteItems(h, dir)
	dirs := make([]Item, 0, len(items))
	for _, item := range items {
		if item.IsDir {
			dirs = append(dirs, item)
		}
	}
	return dirs
}

func parseRemoteItems(out string) []Item {
	var items []Item
	seen := map[string]bool{}
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		if seen[parts[1]] {
			continue
		}
		seen[parts[1]] = true
		items = append(items, Item{Path: parts[1], IsDir: parts[0] == "D"})
	}
	sortItems(items)
	return items
}

func runSSH(h host.Host, script string) (string, error) {
	args, target, cleanup := sshconfig.SSHArgs(h,
		"-o", "ConnectTimeout=3",
		"-o", "LogLevel=ERROR",
	)
	defer cleanup()
	args = append(args, target, "sh", "-s")
	if fullArgs, passCleanup, ok := sshconfig.SSHPassArgs(h.Password, "ssh", sshconfig.PasswordAuthArgs(h), args); ok {
		defer passCleanup()
		cmd := exec.Command("sshpass", fullArgs...)
		cmd.Stdin = strings.NewReader(script)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	cmd := exec.Command("ssh", args...)
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func sortItems(items []Item) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return strings.ToLower(items[i].Path) < strings.ToLower(items[j].Path)
	})
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
