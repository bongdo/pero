package workspace

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func checkPositions(t *testing.T, match Match, query string) {
	t.Helper()
	rel := []rune(match.File.Rel)
	want := foldRunes([]rune(query))
	if len(match.Positions) != len(want) {
		t.Fatalf("позиций %d, want %d: %v", len(match.Positions), len(want), match.Positions)
	}
	last := -1
	for i, pos := range match.Positions {
		if pos <= last || pos >= len(rel) {
			t.Fatalf("позиция %d вне порядка/строки: %v", pos, match.Positions)
		}
		if got := foldRune(rel[pos]); got != want[i] {
			t.Fatalf("позиция %d указывает на %q, want %q", pos, got, want[i])
		}
		last = pos
	}
}

func TestFilterCyrillic(t *testing.T) {
	files := []File{
		{Path: "/p/docs/Пример.md", Rel: "docs/Пример.md"},
		{Path: "/p/src/программа.go", Rel: "src/программа.go"},
		{Path: "/p/Ёлка.txt", Rel: "Ёлка.txt"},
	}
	matches := Filter(files, "прм", 0)
	if len(matches) == 0 {
		t.Fatal("кириллица не нашлась")
	}
	if got := matches[0].File.Rel; got != "docs/Пример.md" {
		t.Fatalf("первое совпадение = %q, want docs/Пример.md", got)
	}
	checkPositions(t, matches[0], "прм")

	for _, query := range []string{"ёлка", "ЁЛКА", "елка", "Елка"} {
		got := Filter(files, query, 0)
		if len(got) != 1 || got[0].File.Rel != "Ёлка.txt" {
			t.Fatalf("Filter(%q) = %v, want Ёлка.txt", query, got)
		}
		checkPositions(t, got[0], query)
	}
}

func TestFilterNameBeforePath(t *testing.T) {
	files := []File{
		{Path: "/p/cmd/x.go", Rel: "cmd/x.go"},
		{Path: "/p/docs/cmd.md", Rel: "docs/cmd.md"},
	}
	matches := Filter(files, "cmd", 0)
	if len(matches) != 2 {
		t.Fatalf("совпадений %d, want 2", len(matches))
	}
	if matches[0].File.Rel != "docs/cmd.md" {
		t.Fatalf("имя должно быть раньше пути: %v", matches[0].File.Rel)
	}
	checkPositions(t, matches[1], "cmd")
}

func TestFilterPrefixWins(t *testing.T) {
	files := []File{
		{Path: "/p/domain.go", Rel: "domain.go"},
		{Path: "/p/main.go", Rel: "main.go"},
	}
	matches := Filter(files, "ma", 0)
	if len(matches) != 2 {
		t.Fatalf("совпадений %d, want 2", len(matches))
	}
	if matches[0].File.Rel != "main.go" {
		t.Fatalf("префикс должен быть первым: %v", rels(matchesFiles(matches)))
	}
}

func TestFilterPositions(t *testing.T) {
	files := []File{{Path: "/p/internal/ui/model.go", Rel: "internal/ui/model.go"}}
	matches := Filter(files, "mdl", 0)
	if len(matches) != 1 {
		t.Fatalf("совпадений %d, want 1", len(matches))
	}
	if want := []int{12, 14, 16}; !slices.Equal(matches[0].Positions, want) {
		t.Fatalf("позиции = %v, want %v", matches[0].Positions, want)
	}

	matches = Filter(files, "iui", 0)
	if len(matches) != 1 {
		t.Fatalf("путь не нашёлся")
	}
	if want := []int{0, 9, 10}; !slices.Equal(matches[0].Positions, want) {
		t.Fatalf("позиции по пути = %v, want %v", matches[0].Positions, want)
	}
}

func TestFilterEmptyQuery(t *testing.T) {
	files := []File{
		{Rel: "one.go"},
		{Rel: "two.go"},
		{Rel: "three.go"},
	}
	matches := Filter(files, "", 2)
	if len(matches) != 2 {
		t.Fatalf("совпадений %d, want 2", len(matches))
	}
	if matches[0].File.Rel != "one.go" || matches[1].File.Rel != "two.go" {
		t.Fatalf("пустой query сломал исходный порядок: %v", rels(matchesFiles(matches)))
	}
	for _, match := range matches {
		if len(match.Positions) != 0 {
			t.Fatalf("у пустого query не должно быть позиций: %v", match.Positions)
		}
	}
}

func TestFilterDefaultLimit(t *testing.T) {
	files := make([]File, 300)
	for i := range files {
		files[i] = File{Rel: fmt.Sprintf("f%03d.txt", i)}
	}
	if got := len(Filter(files, "f", 0)); got != defaultFuzzyLimit {
		t.Fatalf("limit по умолчанию = %d, want %d", got, defaultFuzzyLimit)
	}
}

func TestFilterNoMatch(t *testing.T) {
	files := []File{{Rel: "main.go"}}
	if matches := Filter(files, "zzz", 0); len(matches) != 0 {
		t.Fatalf("нашлось лишнее: %v", matches)
	}
}

func TestFilterDeterministic(t *testing.T) {
	files := []File{
		{Path: "/p/b/one.txt", Rel: "b/one.txt"},
		{Path: "/p/a/one.txt", Rel: "a/one.txt"},
	}
	first := Filter(files, "one", 0)
	second := Filter([]File{files[0], files[1]}, "one", 0)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("Filter недетерминирован:\n%v\n%v", first, second)
	}
	if first[0].File.Rel != "a/one.txt" {
		t.Fatalf("тай-брейк по Rel не работает: %v", rels(matchesFiles(first)))
	}
}

func matchesFiles(matches []Match) []File {
	files := make([]File, len(matches))
	for i, match := range matches {
		files[i] = match.File
	}
	return files
}
