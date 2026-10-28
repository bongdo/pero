package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestStateRoundTrip(t *testing.T) {
	_, statePath := testEnv(t)

	want := State{
		Tabs: []TabState{
			{Path: "~/notes.md", CursorLine: 12, CursorCol: 4},
			{Path: "/tmp/main.go", CursorLine: 0, CursorCol: 0},
		},
		Recent:         []string{"notes.md", "main.go"},
		RecentCommands: []string{"save", "find"},
		LastFile:       "notes.md",
	}
	if err := SaveState(want); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	got, ok := LoadState()
	if !ok {
		t.Fatal("LoadState не нашёл сохранённый state")
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("state round-trip:\n got %+v\nwant %+v", got, want)
	}
	assertMode(t, statePath, 0o644)

	// В каталоге только state.toml: временные файлы убраны.
	entries, err := os.ReadDir(filepath.Dir(statePath))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "state.toml" {
		t.Fatalf("лишние файлы после сохранения: %v", entries)
	}

	// Перезапись поверх файла с другими правами снова даёт 0644.
	if err := os.Chmod(statePath, 0o600); err != nil {
		t.Fatal(err)
	}
	want.LastFile = "main.go"
	if err := SaveState(want); err != nil {
		t.Fatalf("повторный SaveState: %v", err)
	}
	assertMode(t, statePath, 0o644)
	got, ok = LoadState()
	if !ok || got.LastFile != "main.go" {
		t.Fatalf("после перезаписи: %+v, %v", got, ok)
	}
}

func TestLoadStateTolerant(t *testing.T) {
	_, statePath := testEnv(t)

	if _, ok := LoadState(); ok {
		t.Fatal("отсутствующий state не должен считаться загруженным")
	}

	writeFile(t, statePath, "tabs = [\n")
	if _, ok := LoadState(); ok {
		t.Fatal("битый state не должен считаться загруженным")
	}

	writeFile(t, statePath, "last_file = \"note.md\"\nrecent = [\"a\", \"b\"]\n")
	st, ok := LoadState()
	if !ok || st.LastFile != "note.md" || !reflect.DeepEqual(st.Recent, []string{"a", "b"}) {
		t.Fatalf("частичный state: %+v, %v", st, ok)
	}

	// Сохранение поверх битого файла чинит его.
	if err := SaveState(State{LastFile: "fixed.md"}); err != nil {
		t.Fatalf("SaveState поверх битого: %v", err)
	}
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "last_file") {
		t.Fatalf("state.toml без last_file:\n%s", data)
	}
}
