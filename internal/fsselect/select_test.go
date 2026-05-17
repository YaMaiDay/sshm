package fsselect

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/YaMaiDay/sshm/internal/remotescript"
)

func TestExpandLocalRoots(t *testing.T) {
	home := filepath.Join(string(os.PathSeparator), "home", "alice")
	got := ExpandLocalRoots(home, []string{".", "~", "~/Downloads", "/tmp"})
	want := []string{".", home, filepath.Join(home, "Downloads"), "/tmp"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ExpandLocalRoots() = %#v, want %#v", got, want)
	}
}

func TestLocalItemsSkipsHiddenAndSortsDirsFirst(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "z.txt"), []byte("z"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".hidden"), []byte("hidden"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "b-dir"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "a-dir"), 0700); err != nil {
		t.Fatal(err)
	}

	items := LocalItems(dir)
	got := itemNames(items)
	want := []string{"a-dir/", "b-dir/", "z.txt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LocalItems() = %#v, want %#v", got, want)
	}
}

func TestParseRemoteItemsDedupesAndSorts(t *testing.T) {
	items := parseRemoteItems("F\t/var/file\nD\t/opt\nD\t/home\nD\t/opt\nbad line\n")
	got := itemNames(items)
	want := []string{"home/", "opt/", "file"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseRemoteItems() = %#v, want %#v", got, want)
	}
}

func TestShellQuote(t *testing.T) {
	got := remotescript.SingleQuote("/tmp/a'b")
	want := `'/tmp/a'\''b'`
	if got != want {
		t.Fatalf("SingleQuote() = %q, want %q", got, want)
	}
}

func itemNames(items []Item) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		name := filepath.Base(item.Path)
		if item.IsDir {
			name += "/"
		}
		out = append(out, name)
	}
	return out
}
