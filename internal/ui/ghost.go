package ui

import (
	"context"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/bongdo/pero/internal/assist"
	"github.com/bongdo/pero/internal/ed"
	"github.com/bongdo/pero/internal/lsp"
)

type ghostTickMsg struct{ seq int }

type ghostMsg struct {
	tab     *Tab
	gen     int
	version int
	pos     ed.Pos
	prefix  string
	items   []lsp.CompletionItem
	err     error
}

// scheduleGhost перезапускает дебаунс: старый тик и старый ответ LSP
// отбрасываются по номеру поколения.
func (m *Model) scheduleGhost() {
	m.cancelGhost()
	if !m.cfg.Editor.Ghost || m.tab() == nil {
		return
	}
	delay := time.Duration(m.cfg.Editor.GhostDelayMS) * time.Millisecond
	if delay <= 0 {
		delay = 250 * time.Millisecond
	}
	seq := m.ghostGen
	m.ghostCmd = tea.Tick(delay, func(time.Time) tea.Msg {
		return ghostTickMsg{seq: seq}
	})
}

// cancelGhost гасит ghost во всех вкладках и отменяет ожидающий тик.
func (m *Model) cancelGhost() {
	m.ghostGen++
	m.ghostCmd = nil
	for _, t := range m.tabs {
		t.ghost = ghostState{}
	}
}

// ghostPrefix возвращает слово перед курсором, если ghost уместен: конец
// слова, нет выделения, файл не read-only (бинарник открывается read-only)
// и никакой оверлей не открыт.
func (m *Model) ghostPrefix(t *Tab) (string, bool) {
	if !m.cfg.Editor.Ghost || t == nil || t != m.tab() {
		return "", false
	}
	if m.ov.kind != overlayNone || t.ed.Buf.ReadOnly || t.ed.HasSelection() {
		return "", false
	}
	cur := t.ed.Cursor
	if cur.Line < 0 || cur.Line > t.ed.Buf.LastLine() {
		return "", false
	}
	line := t.ed.Buf.Line(cur.Line)
	if cur.Col < 0 || cur.Col > len(line) {
		return "", false
	}
	// Середина слова: там работает обычный completion, не ghost.
	if cur.Col < len(line) && assist.IsWordRune(line[cur.Col]) {
		return "", false
	}
	prefix := assist.WordPrefix(line, cur.Col)
	if prefix == "" {
		return "", false
	}
	return prefix, true
}

func (m *Model) onGhostTick(msg ghostTickMsg) tea.Cmd {
	if msg.seq != m.ghostGen {
		return nil
	}
	t := m.tab()
	prefix, ok := m.ghostPrefix(t)
	if !ok {
		return nil
	}
	return m.ghostSuggest(t, prefix)
}

// ghostSuggest выбирает один источник: prose — t9, code — LSP. Чужие
// варианты не смешиваются.
func (m *Model) ghostSuggest(t *Tab, prefix string) tea.Cmd {
	if m.isProsePath(t.ed.Buf.Path) {
		if g, ok := ghostFrom(assist.SourceT9, prefix, m.wordIndex(t).Candidates(prefix, 20)); ok {
			g.version = t.ed.Buf.Version
			t.ghost = g
		}
		return nil
	}

	cur := t.ed.Cursor
	line := t.ed.Buf.Line(cur.Line)
	start, end := wordBounds(line, cur.Col)
	if len(t.comp.all) > 0 && t.comp.prefix == prefix &&
		t.comp.start == (ed.Pos{Line: cur.Line, Col: start}) &&
		t.comp.end == (ed.Pos{Line: cur.Line, Col: end}) {
		if g, ok := ghostFrom(assist.SourceLSP, prefix, completionTexts(t.comp.all)); ok {
			g.version = t.ed.Buf.Version
			t.ghost = g
		}
		return nil
	}
	if !m.lspReadyFor(t) || !t.lsp.Supports("completion") {
		return nil
	}

	client, uri := t.lsp, t.lspURI
	version := t.ed.Buf.Version
	gen := m.ghostGen
	pos := cur
	lpos := lsp.Position{Line: pos.Line, Character: t.ed.Buf.UTF16Offset(pos)}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), completionTimeout)
		defer cancel()
		items, err := client.Completion(ctx, uri, lpos)
		return ghostMsg{tab: t, gen: gen, version: version, pos: pos, prefix: prefix, items: items, err: err}
	}
}

func (m *Model) onGhostResult(msg ghostMsg) tea.Cmd {
	if msg.gen != m.ghostGen || !m.tabAlive(msg.tab) || msg.tab != m.tab() {
		return nil
	}
	if msg.tab.ed.Buf.Version != msg.version || msg.tab.ed.Cursor != msg.pos {
		return nil
	}
	prefix, ok := m.ghostPrefix(msg.tab)
	if !ok || prefix != msg.prefix {
		return nil
	}
	if msg.err != nil {
		m.failLSP(msg.tab, msg.err)
		return nil
	}
	if g, ok := ghostFrom(assist.SourceLSP, prefix, completionTexts(msg.items)); ok {
		g.version = msg.version
		msg.tab.ghost = g
	}
	return nil
}

func ghostFrom(source, prefix string, candidates []string) (ghostState, bool) {
	s, ok := assist.GhostCandidateFrom(source, prefix, candidates, 2)
	if !ok {
		return ghostState{}, false
	}
	text := ghostSuffix(prefix, s.Text)
	if text == "" {
		return ghostState{}, false
	}
	return ghostState{text: text, source: s.Source}, true
}

// ghostSuffix — только то, что реально дописывается: полное слово минус
// уже набранный префикс.
func ghostSuffix(prefix, candidate string) string {
	pr, cr := []rune(prefix), []rune(candidate)
	if len(cr) <= len(pr) || !strings.EqualFold(string(cr[:len(pr)]), prefix) {
		return ""
	}
	return string(cr[len(pr):])
}

func completionTexts(items []lsp.CompletionItem) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		text := it.InsertText
		if it.TextEdit != nil && it.TextEdit.NewText != "" {
			text = it.TextEdit.NewText
		}
		if text == "" {
			text = it.Label
		}
		out = append(out, text)
	}
	return out
}
