package workspace

import (
	"reflect"
	"strings"
	"testing"
)

func TestDisplayName(t *testing.T) {
	tests := map[string]string{
		"/p/internal/ui/model.go": "model.go",
		"docs/Проект.md":          "Проект.md",
		"README.md":               "README.md",
	}
	for path, want := range tests {
		if got := DisplayName(path); got != want {
			t.Errorf("DisplayName(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestDisambiguate(t *testing.T) {
	paths := []string{
		"/p/pero/README.md",
		"/p/felix-tui/README.md",
		"relative/README.md",
		"/p/pero/main.go",
	}
	got := Disambiguate(paths)
	want := map[string]string{
		"/p/pero/README.md":      "pero/README.md",
		"/p/felix-tui/README.md": "felix-tui/README.md",
		"relative/README.md":     "relative/README.md",
		"/p/pero/main.go":        "main.go",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Disambiguate = %v, want %v", got, want)
	}
	for _, name := range got {
		if strings.HasPrefix(name, "/") {
			t.Fatalf("в имя попал абсолютный путь: %q", name)
		}
	}
}

func TestDisambiguateDeeperSuffix(t *testing.T) {
	got := Disambiguate([]string{"/a/pkg/README.md", "/b/pkg/README.md"})
	want := map[string]string{
		"/a/pkg/README.md": "a/pkg/README.md",
		"/b/pkg/README.md": "b/pkg/README.md",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Disambiguate = %v, want %v", got, want)
	}
}

func TestDisambiguateDuplicatesAndEmpty(t *testing.T) {
	got := Disambiguate([]string{"/a/x.md", "/a/x.md"})
	if len(got) != 1 || got["/a/x.md"] != "x.md" {
		t.Fatalf("дубликат пути: %v", got)
	}
	if empty := Disambiguate(nil); len(empty) != 0 {
		t.Fatalf("Disambiguate(nil) = %v", empty)
	}
}
