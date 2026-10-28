package ui

import (
	"strings"
	"testing"

	"github.com/bongdo/pero/internal/ed"
	"github.com/bongdo/pero/internal/lsp"
)

func TestRenderEmptyAndUnicode(t *testing.T) {
	for _, tc := range []struct{ w, h int }{{40, 10}, {80, 24}, {160, 50}} {
		m := testModel(t)
		setSize(m, tc.w, tc.h)
		viewLines(t, m)
		typeText(m, "привет, ёжик 😀 - test\n\tтаб")
		viewLines(t, m)
		if !strings.Contains(text(m), "ёжик") {
			t.Fatalf("unicode потерялся: %q", text(m))
		}
	}
}

func TestRenderEmptyBufferSmall(t *testing.T) {
	m := testModel(t)
	for _, sz := range []struct{ w, h int }{{1, 1}, {10, 3}, {39, 9}, {40, 10}} {
		setSize(m, sz.w, sz.h)
		viewLines(t, m)
	}
}

func TestReadOnlyBlocksTyping(t *testing.T) {
	m := testModel(t)
	m.tab().ed.Buf.ReadOnly = true
	before := text(m)
	typeText(m, "x")
	if text(m) != before {
		t.Fatal("read-only буфер изменён")
	}
	if m.toast.Kind != "error" {
		t.Fatalf("нет ошибки: %+v", m.toast)
	}
}

func TestGhostNotInBuffer(t *testing.T) {
	m := testModel(t)
	typeText(m, "слово")
	m.tab().ghost = ghostState{text: "подсказка", source: "t9"}
	viewLines(t, m)
	if strings.Contains(text(m), "подсказка") {
		t.Fatal("ghost попал в буфер")
	}
	// Tab принимает заготовку ghost.
	m.Update(keyMsg('\t', "", 0))
	if !strings.Contains(text(m), "подсказка") {
		t.Fatalf("ghost не принят: %q", text(m))
	}
}

func TestDiagnosticsRenderAndCounter(t *testing.T) {
	m := testModel(t)
	typeText(m, "ошибка тут")
	m.tab().diags = []lsp.Diagnostic{{
		Range:    lsp.Range{Start: lsp.Position{Line: 0, Character: 0}, End: lsp.Position{Line: 0, Character: 6}},
		Severity: 1,
		Message:  "тест",
	}}
	m.tab().diagVersion = m.tab().ed.Buf.Version
	setSize(m, 80, 24)
	viewLines(t, m)
	status := m.statusRow()
	if !strings.Contains(status, "1 ошибка") {
		t.Fatalf("нет счётчика: %q", status)
	}
	m.jumpDiagnostic(1)
	if m.tab().ed.Cursor != (ed.Pos{}) {
		t.Fatalf("F8 не перешёл: %+v", m.tab().ed.Cursor)
	}
}

func TestRenderControlChars(t *testing.T) {
	m := testModel(t)
	m.tab().ed = ed.NewEditor(ed.OpenBytes([]byte("hello\x1b[2Jworld\x00\x07"), "control.txt"))
	m.tab().wrapCache.valid = false
	setSize(m, 40, 10)
	v := m.View()
	if strings.Contains(v.Content, "\x1b[2J") {
		t.Fatal("управляющая последовательность из файла попала в терминал")
	}
	if strings.ContainsRune(v.Content, 0) {
		t.Fatal("NUL попал в вывод")
	}
	viewLines(t, m)
}

func TestFindHighlightUsesBackground(t *testing.T) {
	m := findModel(t, "foo bar")
	setSize(m, 40, 10)
	m.openFind(false)
	m.ov.input.Set("foo")
	m.recomputeFind()
	v := m.View()
	if !strings.Contains(v.Content, "48;2;219;188;127") {
		t.Fatal("текущее совпадение не подсвечено фоном SearchCurrent")
	}
}

func TestStatusCompactsAt40(t *testing.T) {
	m := testModel(t)
	setSize(m, 40, 10)
	viewLines(t, m)
	status := m.statusRow()
	if w := ansiWidth(status); w != 40 {
		t.Fatalf("статус шириной %d", w)
	}
	if !strings.Contains(status, "1:1") {
		t.Fatalf("нет позиции: %q", status)
	}
}

func TestSelectionRenderNoPanic(t *testing.T) {
	m := testModel(t)
	typeText(m, "первая строка\nвторая строка")
	m.tab().ed.SetSelection(ed.Pos{Line: 0, Col: 3}, ed.Pos{Line: 1, Col: 2})
	setSize(m, 40, 10)
	viewLines(t, m)
}

func TestHelpNoPanic(t *testing.T) {
	for _, sz := range []struct{ w, h int }{{40, 10}, {80, 24}, {160, 50}, {1, 1}} {
		m := testModel(t)
		setSize(m, sz.w, sz.h)
		m.openHelp()
		viewLines(t, m)
		pressEsc(m)
		if m.ov.kind != overlayNone {
			t.Fatalf("%dx%d: help не закрылся", sz.w, sz.h)
		}
	}
}

func TestHelpListsRegisteredKeys(t *testing.T) {
	m := testModel(t)
	lines := strings.Join(m.helpLines(80), "\n")
	if !strings.Contains(lines, "Ctrl-S") || !strings.Contains(lines, "Сохранить файл") {
		t.Fatalf("help без Ctrl-S:\n%s", lines)
	}
	if !strings.Contains(lines, "Alt-Q") {
		t.Fatalf("help без Alt-Q:\n%s", lines)
	}
}

func TestCommandPaletteRowsFit(t *testing.T) {
	m := testModel(t)
	for _, sz := range []struct{ w, h int }{{40, 10}, {80, 24}, {160, 50}} {
		setSize(m, sz.w, sz.h)
		m.openCommandPalette()
		viewLines(t, m)
	}
	m.closeOverlays()
}

func TestConfirmNoPanic(t *testing.T) {
	m := testModel(t)
	typeText(m, "x")
	for _, sz := range []struct{ w, h int }{{40, 10}, {80, 24}, {20, 8}} {
		setSize(m, sz.w, sz.h)
		m.runCommand("app.quit")
		viewLines(t, m)
		pressEsc(m)
	}
}

func TestConfirmButtonsFit40(t *testing.T) {
	m := testModel(t)
	typeText(m, "x")
	setSize(m, 40, 10)
	m.runCommand("app.quit")
	v := m.confirmView()
	if len(v.buttons) != 3 {
		t.Fatalf("кнопок %d, ожидалось 3", len(v.buttons))
	}
	for _, r := range v.buttons {
		if r.x < 0 || r.x+r.w > m.w {
			t.Fatalf("кнопка за экраном: %+v", r)
		}
	}
}
