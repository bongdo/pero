package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bongdo/pero/internal/ed"
)

func findModel(t *testing.T, content string) *Model {
	t.Helper()
	m := testModel(t)
	m.tab().ed = ed.NewEditor(ed.OpenBytes([]byte(content), "find.md"))
	m.tab().ed.TabWidth = 4
	m.tab().hlVersion = -1
	m.tab().wrapCache.valid = false
	return m
}

func TestFindLiveAndStep(t *testing.T) {
	m := findModel(t, "foo bar foo\nfoo")
	m.openFind(false)
	m.ov.input.Set("foo")
	m.recomputeFind()
	if len(m.ov.findMatches) != 3 {
		t.Fatalf("совпадений %d", len(m.ov.findMatches))
	}
	cursor := m.tab().ed.Cursor
	m.findStep(1)
	if !m.tab().ed.HasSelection() {
		t.Fatal("Enter не выделил совпадение")
	}
	if m.tab().ed.Cursor == cursor {
		t.Fatal("курсор не сдвинулся по Enter")
	}
	// Ввод нового запроса не двигает курсор.
	before := m.tab().ed.Cursor
	m.ov.input.Set("bar")
	m.recomputeFind()
	if m.tab().ed.Cursor != before {
		t.Fatalf("ввод подвинул курсор")
	}
}

func TestFindDoesNotMoveUntilEnter(t *testing.T) {
	m := findModel(t, "aaa aaa")
	m.tab().ed.SetCursor(ed.Pos{Line: 0, Col: 4})
	before := m.tab().ed.Cursor
	m.openFind(false)
	m.ov.input.Set("aaa")
	m.recomputeFind()
	if m.tab().ed.Cursor != before {
		t.Fatal("live-поиск сдвинул курсор")
	}
}

func TestFindCaseInsensitiveCyrillic(t *testing.T) {
	m := findModel(t, "Привет, привет и ПРИВЕТ")
	m.openFind(false)
	m.ov.input.Set("привет")
	m.recomputeFind()
	if len(m.ov.findMatches) != 3 {
		t.Fatalf("совпадений %d", len(m.ov.findMatches))
	}
}

func TestReplaceAllOneTransaction(t *testing.T) {
	m := findModel(t, "foo bar foo\nfoo")
	m.openFind(true)
	m.ov.input.Set("foo")
	m.ov.input2.Set("baz")
	m.recomputeFind()
	m.replaceAll()
	if got := text(m); got != "baz bar baz\nbaz" {
		t.Fatalf("после замены %q", got)
	}
	if !m.tab().ed.Undo() {
		t.Fatal("undo недоступен")
	}
	if got := text(m); got != "foo bar foo\nfoo" {
		t.Fatalf("undo вернул %q", got)
	}
}

func TestReplaceOne(t *testing.T) {
	m := findModel(t, "foo foo")
	m.openFind(true)
	m.ov.input.Set("foo")
	m.ov.input2.Set("x")
	m.recomputeFind()
	m.ov.cur = 0
	m.replaceOne()
	if got := text(m); got != "x foo" {
		t.Fatalf("после замены %q", got)
	}
}

func TestFindRowsLayout(t *testing.T) {
	m := findModel(t, "foo foo")
	setSize(m, 40, 10)
	m.openFind(true)
	m.ov.input.Set("foo")
	m.recomputeFind()
	viewLines(t, m)
	if lay := m.layout(); lay.findH != 2 {
		t.Fatalf("строк поиска %d", lay.findH)
	}
	setSize(m, 80, 24)
	viewLines(t, m)
}

func TestFindUndoStaysInOverlay(t *testing.T) {
	m := findModel(t, "foo bar foo")
	m.openFind(true)
	m.ov.input.Set("foo")
	m.ov.input2.Set("baz")
	m.recomputeFind()
	m.replaceAll()
	m.Update(keyMsg('z', "", tea.ModCtrl))
	if got := text(m); got != "foo bar foo" {
		t.Fatalf("Ctrl-Z в поиске не откатил: %q", got)
	}
	if m.ov.kind != overlayFind {
		t.Fatal("Ctrl-Z закрыл поиск")
	}
	if len(m.ov.findMatches) != 2 {
		t.Fatalf("счётчик не пересчитан: %d", len(m.ov.findMatches))
	}
}
