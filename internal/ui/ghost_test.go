package ui

import (
	"strings"
	"testing"

	"github.com/bongdo/pero/internal/assist"
	"github.com/bongdo/pero/internal/ed"
	"github.com/bongdo/pero/internal/lsp"
	"github.com/charmbracelet/x/ansi"
)

// ghostAfter ставит курсор, перезапускает дебаунс и прогоняет тик без ожидания.
func ghostAfter(m *Model, line, col int) ghostState {
	tab := m.tab()
	tab.ed.SetCursor(ed.Pos{Line: line, Col: col})
	m.scheduleGhost()
	m.Update(ghostTickMsg{seq: m.ghostGen})
	return tab.ghost
}

func TestGhostDebounceSeq(t *testing.T) {
	m, _ := testModelFile(t, "note.md", "приве приветствие приветствие")
	tab := m.tab()
	tab.ed.SetCursor(ed.Pos{Line: 0, Col: 5})
	m.scheduleGhost()
	old := m.ghostGen
	m.scheduleGhost()
	m.Update(ghostTickMsg{seq: old})
	if tab.ghost.text != "" {
		t.Fatalf("старый тик показал ghost: %q", tab.ghost.text)
	}
	m.Update(ghostTickMsg{seq: m.ghostGen})
	if tab.ghost.text != "тствие" {
		t.Fatalf("ghost: %q, want %q", tab.ghost.text, "тствие")
	}
	if tab.ghost.source != assist.SourceT9 {
		t.Fatalf("источник: %q", tab.ghost.source)
	}
	if tab.ghost.version != tab.ed.Buf.Version {
		t.Fatalf("версия: %d, want %d", tab.ghost.version, tab.ed.Buf.Version)
	}
}

func TestGhostIndexLazy(t *testing.T) {
	m, _ := testModelFile(t, "note.md", "приве приветствие приветствие")
	tab := m.tab()
	if tab.wordsVer != -1 {
		t.Fatalf("индекс собран до запроса: %d", tab.wordsVer)
	}
	if g := ghostAfter(m, 0, 5); g.text == "" {
		t.Fatal("t9 не дал ghost")
	}
	if tab.wordsVer != tab.ed.Buf.Version {
		t.Fatal("индекс не совпал с версией буфера")
	}
	indexed := tab.wordsVer
	typeText(m, "!")
	if tab.ed.Buf.Version == indexed || tab.wordsVer != indexed {
		t.Fatalf("индекс пересобрался на ввод: ver=%d words=%d", tab.ed.Buf.Version, tab.wordsVer)
	}
}

func TestGhostFilters(t *testing.T) {
	m, _ := testModelFile(t, "note.md", "привет мир")
	if g := ghostAfter(m, 0, 3); g.text != "" {
		t.Fatalf("середина слова: %q", g.text)
	}
	if g := ghostAfter(m, 0, 6); g.text != "" {
		t.Fatalf("ghost равен префиксу: %q", g.text)
	}
	if g := ghostAfter(m, 0, 10); g.text != "" {
		t.Fatalf("ghost равен префиксу в конце строки: %q", g.text)
	}
	m.tab().ed.SelectAll()
	m.scheduleGhost()
	m.Update(ghostTickMsg{seq: m.ghostGen})
	if m.tab().ghost.text != "" {
		t.Fatal("ghost при выделении")
	}
}

func TestGhostFromFilters(t *testing.T) {
	if _, ok := ghostFrom(assist.SourceT9, "при", []string{"пр"}); ok {
		t.Fatal("короткий кандидат прошёл")
	}
	if _, ok := ghostFrom(assist.SourceT9, "при", []string{"при"}); ok {
		t.Fatal("равный кандидат прошёл")
	}
	g, ok := ghostFrom(assist.SourceT9, "при", []string{"привет"})
	if !ok || g.text != "вет" {
		t.Fatalf("суффикс: %+v, ok=%v", g, ok)
	}
	if _, ok := ghostFrom(assist.SourceT9, "", []string{"привет"}); ok {
		t.Fatal("пустой префикс прошёл")
	}
}

func TestGhostAcceptOneUndo(t *testing.T) {
	m, _ := testModelFile(t, "note.md", "приве приветствие приветствие")
	if g := ghostAfter(m, 0, 5); g.text == "" {
		t.Fatal("ghost не появился")
	}
	want := "приветствие приветствие приветствие"
	m.Update(keyMsg('\t', "", 0))
	if got := text(m); got != want {
		t.Fatalf("после Tab: %q", got)
	}
	if m.tab().ghost.text != "" {
		t.Fatal("ghost не погас после принятия")
	}
	pressCtrl(m, 'z')
	if got := text(m); got != "приве приветствие приветствие" {
		t.Fatalf("undo: %q", got)
	}
}

func TestGhostEscNoBuffer(t *testing.T) {
	m, _ := testModelFile(t, "note.md", "приве приветствие приветствие")
	if g := ghostAfter(m, 0, 5); g.text == "" {
		t.Fatal("ghost не появился")
	}
	seq := m.ghostGen
	pressEsc(m)
	if m.tab().ghost.text != "" {
		t.Fatal("Esc не погасил ghost")
	}
	if got := text(m); got != "приве приветствие приветствие" {
		t.Fatalf("Esc изменил буфер: %q", got)
	}
	m.Update(ghostTickMsg{seq: seq})
	if m.tab().ghost.text != "" {
		t.Fatal("Esc не отменил ожидающий тик")
	}
}

func TestGhostTabSwitchNoLeak(t *testing.T) {
	m, _ := testModelFile(t, "note.md", "приве приветствие приветствие")
	if g := ghostAfter(m, 0, 5); g.text == "" {
		t.Fatal("ghost не появился")
	}
	old := m.tabs[0]
	m.newEmptyTab()
	if old.ghost.text != "" {
		t.Fatal("ghost старой вкладки остался")
	}
	if m.tab().ghost.text != "" {
		t.Fatal("ghost протёк в новую вкладку")
	}
	for _, line := range viewLines(t, m) {
		if strings.Contains(line, "тствие") {
			t.Fatalf("ghost старой вкладки виден: %q", line)
		}
	}
}

func TestGhostDisabled(t *testing.T) {
	m, _ := testModelFile(t, "note.md", "приве приветствие приветствие")
	m.cfg.Editor.Ghost = false
	if g := ghostAfter(m, 0, 5); g.text != "" {
		t.Fatalf("ghost при ghost=false: %q", g.text)
	}
	m.tab().ghost = ghostState{text: "x", source: "t9"}
	m.runCommand("view.ghost")
	if !m.cfg.Editor.Ghost {
		t.Fatal("view.ghost не включил обратно")
	}
	if m.tab().ghost.text != "" {
		t.Fatal("ghost не очищен при выключении")
	}
}

func TestGhostLSPReusesCompletion(t *testing.T) {
	m, _ := testModelFile(t, "main.go", "x ma")
	tab := m.tab()
	tab.ed.SetCursor(ed.Pos{Line: 0, Col: 4})
	tab.comp = completionState{
		prefix: "ma",
		start:  ed.Pos{Line: 0, Col: 2},
		end:    ed.Pos{Line: 0, Col: 4},
		all:    []lsp.CompletionItem{{Label: "main", InsertText: "main"}},
	}
	m.scheduleGhost()
	m.Update(ghostTickMsg{seq: m.ghostGen})
	if tab.ghost.text != "in" || tab.ghost.source != assist.SourceLSP {
		t.Fatalf("lsp ghost: %+v", tab.ghost)
	}
}

func TestGhostProseDoesNotUseLSP(t *testing.T) {
	m, _ := testModelFile(t, "note.md", "пр")
	tab := m.tab()
	tab.comp = completionState{
		prefix: "пр",
		start:  ed.Pos{Line: 0, Col: 0},
		end:    ed.Pos{Line: 0, Col: 2},
		all:    []lsp.CompletionItem{{Label: "проект", InsertText: "проект"}},
	}
	m.scheduleGhost()
	m.Update(ghostTickMsg{seq: m.ghostGen})
	if tab.ghost.text != "" {
		t.Fatalf("в prose показан LSP: %+v", tab.ghost)
	}
}

func TestGhostLSPResultStale(t *testing.T) {
	m, _ := testModelFile(t, "main.go", "x ma")
	tab := m.tab()
	tab.ed.SetCursor(ed.Pos{Line: 0, Col: 4})
	m.scheduleGhost()
	msg := ghostMsg{
		tab: tab, gen: m.ghostGen, version: tab.ed.Buf.Version - 1,
		pos: ed.Pos{Line: 0, Col: 4}, prefix: "ma",
		items: []lsp.CompletionItem{{InsertText: "main"}},
	}
	m.Update(msg)
	if tab.ghost.text != "" {
		t.Fatal("старая версия показала ghost")
	}
	msg.version = tab.ed.Buf.Version
	msg.prefix = "m"
	m.Update(msg)
	if tab.ghost.text != "" {
		t.Fatal("другой префикс показал ghost")
	}
	msg.prefix = "ma"
	m.Update(msg)
	if tab.ghost.text != "in" {
		t.Fatalf("свежий ответ: %q", tab.ghost.text)
	}
}

func TestGhostRenderSuffixMidLine(t *testing.T) {
	m, _ := testModelFile(t, "note.md", "приве приветствие мир")
	if g := ghostAfter(m, 0, 5); g.text != "тствие" {
		t.Fatalf("ghost: %q", g.text)
	}
	if got := text(m); got != "приве приветствие мир" {
		t.Fatalf("ghost попал в буфер: %q", got)
	}
	found := false
	for _, line := range viewLines(t, m) {
		if strings.Contains(ansi.Strip(line), "приветствие приветствие мир") {
			found = true
		}
	}
	if !found {
		t.Fatalf("ghost не нарисован суффиксом: %q", strings.Join(viewLines(t, m), "\n"))
	}
}

func TestGhostReadOnly(t *testing.T) {
	m, _ := testModelFile(t, "note.md", "приве приветствие приветствие")
	m.tab().ed.Buf.ReadOnly = true
	if g := ghostAfter(m, 0, 5); g.text != "" {
		t.Fatalf("ghost в read-only: %q", g.text)
	}
}

func TestGhostHiddenByOverlayAndTyping(t *testing.T) {
	m, _ := testModelFile(t, "note.md", "приве приветствие приветствие")
	if g := ghostAfter(m, 0, 5); g.text == "" {
		t.Fatal("ghost не появился")
	}
	typeText(m, "!")
	if m.tab().ghost.text != "" {
		t.Fatal("печать не погасила ghost")
	}
	m.tab().ghost = ghostState{text: "тствие", source: assist.SourceT9}
	m.ov.kind = overlayHelp
	for _, line := range viewLines(t, m) {
		if strings.Contains(line, "тствие") {
			t.Fatalf("ghost виден под оверлеем: %q", line)
		}
	}
	m.scheduleGhost()
	m.Update(ghostTickMsg{seq: m.ghostGen})
	if m.tab().ghost.text != "" {
		t.Fatal("тик показал ghost при открытом оверлее")
	}
}

func TestGhostAcceptBlockedByCompletionPopup(t *testing.T) {
	m, _ := testModelFile(t, "note.md", "приве приветствие приветствие")
	if g := ghostAfter(m, 0, 5); g.text == "" {
		t.Fatal("ghost не появился")
	}
	tab := m.tab()
	tab.comp = completionState{
		open:  true,
		items: []lsp.CompletionItem{{Label: "приветик", InsertText: "приветик"}},
		all:   []lsp.CompletionItem{{Label: "приветик", InsertText: "приветик"}},
		start: ed.Pos{Line: 0, Col: 0},
		end:   ed.Pos{Line: 0, Col: 5},
	}
	m.ov.kind = overlayCompletion
	m.Update(keyMsg('\t', "", 0))
	if got := text(m); got != "приветик приветствие приветствие" {
		t.Fatalf("Tab принял не popup: %q", got)
	}
}
