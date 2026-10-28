package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestRecentMissingFile(t *testing.T) {
	recent, err := LoadRecent(filepath.Join(t.TempDir(), "state.toml"))
	if err != nil {
		t.Fatalf("нет файла, а ошибка: %v", err)
	}
	if recent == nil || len(recent) != 0 {
		t.Fatalf("LoadRecent = %#v, want пустой список", recent)
	}
}

func TestRecentRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.toml")
	want := []string{"/p/main.go", "/p/док.md"}
	if err := SaveRecent(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadRecent(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadRecent = %v, want %v", got, want)
	}
	if _, err := os.Stat(path + ".tmp"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("временный файл остался: %v", err)
	}
}

func TestRecentEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.toml")
	if err := SaveRecent(path, nil); err != nil {
		t.Fatal(err)
	}
	got, err := LoadRecent(path)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("LoadRecent = %#v, want пустой список", got)
	}
}

func TestRecentCreatesDirs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deep", "dir", "state.toml")
	if err := SaveRecent(path, []string{"/p/a.go"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestRecentKeepsOtherState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.toml")
	body := "tabs = [{ path = \"/p/a.go\", cursor_line = 3, cursor_col = 1 }]\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SaveRecent(path, []string{"/p/b.go"}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var state struct {
		Tabs []struct {
			Path       string `toml:"path"`
			CursorLine int    `toml:"cursor_line"`
		} `toml:"tabs"`
		Recent []string `toml:"recent"`
	}
	if err := toml.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	if len(state.Tabs) != 1 || state.Tabs[0].Path != "/p/a.go" || state.Tabs[0].CursorLine != 3 {
		t.Fatalf("tabs потерялись: %+v", state.Tabs)
	}
	if !reflect.DeepEqual(state.Recent, []string{"/p/b.go"}) {
		t.Fatalf("recent = %v", state.Recent)
	}
}

func TestRecentBrokenTOML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.toml")
	if err := os.WriteFile(path, []byte("recent = ["), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRecent(path); err == nil {
		t.Fatal("битый TOML прочитан без ошибки")
	}
	if err := SaveRecent(path, []string{"/p/a.go"}); err == nil {
		t.Fatal("битый TOML перетёрт без ошибки")
	}
}

func TestRecentConcurrentSaves(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.toml")
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := range 8 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- SaveRecent(path, []string{fmt.Sprintf("/p/%d.go", i)})
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("параллельный SaveRecent: %v", err)
		}
	}
	recent, err := LoadRecent(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 1 {
		t.Fatalf("recent = %v, want одно значение", recent)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != "state.toml" {
			t.Fatalf("остался временный файл %q", entry.Name())
		}
	}
}
