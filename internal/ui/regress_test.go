package ui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bongdo/pero/internal/ed"
	"github.com/bongdo/pero/internal/lsp"
)

// P0: неудачный Save As не должен оставлять pendingClose.
func TestSaveAsFailureClearsPendingClose(t *testing.T) {
	m := testModel(t)
	typeText(m, "черновик")
	m.pendingClose = 0
	bad := filepath.Join(t.TempDir(), "нет-такой-папки", "f.md")
	m.saveAs(bad)
	if m.pendingClose != -1 {
		t.Fatalf("pendingClose остался %d после ошибки Save As", m.pendingClose)
	}
}

// P0: ответ LSP не закрывает чужой диалог (confirm/Save As/поиск).
func TestLSPResponseKeepsConfirmOverlay(t *testing.T) {
	m := testModel(t)
	typeText(m, "x")
	m.openConfirm(confirmQuit, -1, "Выйти?", "Есть правки")
	m.compGen = 7
	m.Update(completionMsg{
		tab: m.tab(), gen: 7, version: m.tab().ed.Buf.Version, pos: m.tab().ed.Cursor,
		items: []lsp.CompletionItem{{Label: "x"}},
	})
	if m.ov.kind != overlayConfirm {
		t.Fatalf("confirm закрыт ответом LSP: overlay=%v", m.ov.kind)
	}

	m.hoverGen = 3
	m.Update(lspHoverMsg{tab: m.tab(), gen: 3, version: m.tab().ed.Buf.Version, hover: &lsp.Hover{Contents: "h"}})
	if m.ov.kind != overlayConfirm {
		t.Fatalf("confirm закрыт hover: overlay=%v", m.ov.kind)
	}
}

// Ctrl-V не должен вставлять в вкладку, на которую переключились.
func TestPasteStaysInOriginalTab(t *testing.T) {
	m := testModel(t)
	first := m.tab()
	m.pasteTab = first
	m.pendingPaste = true
	m.newEmptyTab()
	second := m.tab()
	if second == first {
		t.Fatal("новая вкладка не создана")
	}
	m.onClipboard("вставка")
	if got := first.ed.Buf.Text(); got != "вставка" {
		t.Fatalf("первая вкладка: %q", got)
	}
	if got := second.ed.Buf.Text(); got != "" {
		t.Fatalf("текст уехал в другую вкладку: %q", got)
	}
}

func TestBracketedPaste(t *testing.T) {
	m := testModel(t)
	m.Update(tea.PasteMsg{Content: "привет\nмир"})
	if got := text(m); got != "привет\nмир" {
		t.Fatalf("bracketed paste: %q", got)
	}
}

// P1: диагностики старой версии не должны рисоваться после правки.
func TestStaleDiagnosticsHidden(t *testing.T) {
	m := testModel(t)
	typeText(m, "ошибка")
	tb := m.tab()
	tb.diags = []lsp.Diagnostic{{
		Range:    lsp.Range{Start: lsp.Position{Line: 0, Character: 0}, End: lsp.Position{Line: 0, Character: 6}},
		Severity: 1,
		Message:  "старая",
	}}
	tb.diagVersion = tb.ed.Buf.Version
	if len(tb.visibleDiags()) == 0 {
		t.Fatal("свежие диагностики скрыты")
	}
	typeText(m, "!")
	if got := tb.visibleDiags(); len(got) != 0 {
		t.Fatalf("диагностики старой версии видны: %v", got)
	}
	if strings.Contains(m.statusRow(), "ошибка") {
		t.Fatal("счётчик показывает старую диагностику")
	}
}

func TestSaveAsDuplicateTabRejected(t *testing.T) {
	m, path := testModelFile(t, "a.md", "a")
	bPath := filepath.Join(filepath.Dir(path), "b.md")
	if err := m.tab().ed.Buf.SaveAs(bPath); err != nil {
		t.Fatal(err)
	}
	m.newEmptyTab()
	m.tab().ed.Insert("x")
	m.saveAs(bPath) // b.md уже открыт в первой вкладке
	if got := m.tab().ed.Buf.Path; got != "" {
		t.Fatalf("Save As перетёр открытую вкладку: %q", got)
	}
}

func TestPageUsesScreenRows(t *testing.T) {
	m := testModel(t)
	m.cfg.Editor.Tabsize = 4
	setSize(m, 40, 10)
	// Длинная markdown-строка переносится на много экранных строк.
	m.tab().wrap = true
	typeText(m, strings.Repeat("слово ", 40))
	m.tab().ed.SetCursor(ed.Pos{})
	before := m.tab().ed.Cursor
	m.moveScreen(1, false)
	if m.tab().ed.Cursor == before {
		t.Fatal("moveScreen не сдвинул курсор")
	}
	m.moveScreen(-1, false)
	if m.tab().ed.Cursor != before {
		t.Fatalf("moveScreen вернул не туда: %+v != %+v", m.tab().ed.Cursor, before)
	}
}
