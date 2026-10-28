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

const completionTimeout = 1500 * time.Millisecond

type completionMsg struct {
	tab     *Tab
	gen     int
	version int
	pos     ed.Pos
	items   []lsp.CompletionItem
	err     error
}

// triggerCompletion - Ctrl-Space. Позиция фиксируется до запроса: ответ на
// изменившийся буфер отбрасывается.
func (m *Model) triggerCompletion() tea.Cmd {
	t := m.tab()
	if t == nil || t.ed.Buf.ReadOnly {
		return nil
	}
	m.scheduleGhost()
	if m.ov.kind == overlayCompletion {
		m.closeOverlays()
	}
	pos := t.ed.Cursor
	line := t.ed.Buf.Line(pos.Line)
	start, end := wordBounds(line, pos.Col)
	prefix := string(line[start:pos.Col])

	m.compGen++
	gen := m.compGen

	if !m.lspReadyFor(t) {
		m.showCompletion(t, gen, m.localCompletions(t, prefix), prefix, start, end)
		if t.lspState == lspStarting || t.lspState == lspFailed || t.lspState == lspOff {
			m.lspOffToast(t)
		}
		return nil
	}
	if !t.lsp.Supports("completion") {
		// Сервер без capabilities не должен лишать пользователя слов из буфера.
		m.showCompletion(t, gen, m.localCompletions(t, prefix), prefix, start, end)
		return nil
	}

	client, uri := t.lsp, t.lspURI
	version := t.ed.Buf.Version
	lpos := lsp.Position{Line: pos.Line, Character: t.ed.Buf.UTF16Offset(pos)}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), completionTimeout)
		defer cancel()
		items, err := client.Completion(ctx, uri, lpos)
		return completionMsg{tab: t, gen: gen, version: version, pos: pos, items: items, err: err}
	}
}

func (m *Model) onCompletion(msg completionMsg) tea.Cmd {
	t := msg.tab
	if !m.tabAlive(t) || t != m.tab() || msg.gen != m.compGen {
		return nil
	}
	if m.ov.kind != overlayNone && m.ov.kind != overlayCompletion {
		return nil
	}
	// Пока сервер думал, текст или курсор могли измениться.
	if t.ed.Buf.Version != msg.version || t.ed.Cursor != msg.pos {
		return nil
	}
	line := t.ed.Buf.Line(msg.pos.Line)
	start, end := wordBounds(line, msg.pos.Col)
	prefix := string(line[start:msg.pos.Col])

	items := msg.items
	if msg.err != nil {
		m.failLSP(t, msg.err)
		// LSP не ответил - не оставляем пользователя без слов из буфера.
		items = m.localCompletions(t, prefix)
	} else if len(items) == 0 {
		items = m.localCompletions(t, prefix)
	}
	m.showCompletion(t, msg.gen, items, prefix, start, end)
	return nil
}

// showCompletion либо сразу подставляет единственного кандидата, либо открывает
// popup. Ноль кандидатов - info-тост, чтобы нажатие не выглядело потерянным.
func (m *Model) showCompletion(t *Tab, gen int, items []lsp.CompletionItem, prefix string, start, end int) {
	items = dedupeCompletions(items)
	if len(items) == 0 {
		t.comp = completionState{}
		if m.ov.kind == overlayCompletion {
			m.closeOverlays()
		}
		m.noCompletionsToast()
		return
	}
	t.comp = completionState{
		open:   true,
		items:  items,
		all:    items,
		prefix: prefix,
		start:  ed.Pos{Line: t.ed.Cursor.Line, Col: start},
		end:    ed.Pos{Line: t.ed.Cursor.Line, Col: end},
		gen:    gen,
	}
	if len(items) == 1 {
		m.acceptCompletion(t, items[0])
		return
	}
	if m.ov.kind != overlayNone && m.ov.kind != overlayCompletion {
		// Пока сервер думал, открылся диалог. Кандидаты сохраняем для ghost,
		// но чужой оверлей не закрываем.
		return
	}
	if m.ov.kind != overlayNone {
		m.closeOverlays()
		// closeOverlays сбрасывает comp текущей вкладки; состояние ниже важнее.
		t.comp = completionState{
			open: true, items: items, all: items, prefix: prefix,
			start: ed.Pos{Line: t.ed.Cursor.Line, Col: start},
			end:   ed.Pos{Line: t.ed.Cursor.Line, Col: end},
			gen:   gen,
		}
	}
	m.ov.kind = overlayCompletion
}

func dedupeCompletions(items []lsp.CompletionItem) []lsp.CompletionItem {
	out := make([]lsp.CompletionItem, 0, len(items))
	seen := make(map[string]bool, len(items))
	for _, it := range items {
		label := strings.TrimSpace(it.Label)
		if label == "" {
			continue
		}
		key := strings.ToLower(label)
		if seen[key] {
			continue
		}
		seen[key] = true
		if it.InsertText == "" {
			it.InsertText = label
		}
		out = append(out, it)
	}
	return out
}

// wordIndex индексирует слова буфера лениво: только при первом запросе после
// правки, а не на каждый keystroke.
func (m *Model) wordIndex(t *Tab) *assist.WordIndex {
	if t == nil {
		return assist.NewWordIndex()
	}
	if t.words == nil {
		t.words = assist.NewWordIndex()
		t.wordsVer = -1
	}
	if t.wordsVer != t.ed.Buf.Version {
		t.words.Index(t.ed.Buf.Text())
		t.wordsVer = t.ed.Buf.Version
	}
	return t.words
}

func (m *Model) localCompletions(t *Tab, prefix string) []lsp.CompletionItem {
	prefix = strings.TrimSpace(prefix)
	if t == nil || prefix == "" {
		return nil
	}
	var out []lsp.CompletionItem
	for _, word := range m.wordIndex(t).Candidates(prefix, 50) {
		if strings.EqualFold(word, prefix) {
			continue
		}
		out = append(out, lsp.CompletionItem{
			Label: word, InsertText: word, Kind: "word", Detail: "t9",
		})
	}
	return out
}

// wordBounds - слово под курсором с учётом середины слова: замена идёт по
// всему слову, иначе хвост остался бы мусором.
func wordBounds(line []rune, col int) (int, int) {
	col = max(0, min(col, len(line)))
	start, end := col, col
	for start > 0 && assist.IsWordRune(line[start-1]) {
		start--
	}
	for end < len(line) && assist.IsWordRune(line[end]) {
		end++
	}
	for start < col && line[start] == '-' {
		start++
	}
	for end > col && line[end-1] == '-' {
		end--
	}
	return start, end
}

func filterCompletions(items []lsp.CompletionItem, query string) []lsp.CompletionItem {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return items
	}
	out := make([]lsp.CompletionItem, 0, len(items))
	for _, it := range items {
		label := strings.ToLower(it.Label)
		if strings.HasPrefix(label, q) || strings.Contains(label, q) {
			out = append(out, it)
		}
	}
	return out
}

func (m *Model) refreshCompletion(t *Tab) {
	if t == nil {
		return
	}
	t.comp.items = filterCompletions(t.comp.all, t.comp.query)
	t.comp.sel = 0
	t.comp.scroll = 0
}

func (m *Model) completionKey(msg tea.KeyPressMsg) tea.Cmd {
	t := m.tab()
	if t == nil {
		m.closeOverlays()
		return nil
	}
	switch msg.Keystroke() {
	case "esc", "ctrl+space", "ctrl+@":
		m.closeOverlays()
		return nil
	case "up", "ctrl+p":
		m.completionMove(-1)
		return nil
	case "down", "ctrl+n":
		m.completionMove(1)
		return nil
	case "pgup":
		m.completionMove(-m.popupHeight())
		return nil
	case "pgdown":
		m.completionMove(m.popupHeight())
		return nil
	case "enter", "tab":
		if len(t.comp.items) > 0 {
			m.acceptCompletion(t, t.comp.items[max(0, min(t.comp.sel, len(t.comp.items)-1))])
		}
		return nil
	case "backspace":
		if t.comp.query == "" {
			m.closeOverlays()
			return nil
		}
		runes := []rune(t.comp.query)
		t.comp.query = string(runes[:len(runes)-1])
		m.refreshCompletion(t)
		return nil
	}
	if printable(msg) {
		t.comp.query += msg.Text
		m.refreshCompletion(t)
	}
	return nil
}

func (m *Model) completionMove(delta int) {
	t := m.tab()
	if t == nil || len(t.comp.items) == 0 {
		return
	}
	n := len(t.comp.items)
	t.comp.sel = max(0, min(t.comp.sel+delta, n-1))
	height := m.popupHeight()
	if t.comp.sel < t.comp.scroll {
		t.comp.scroll = t.comp.sel
	}
	if t.comp.sel >= t.comp.scroll+height {
		t.comp.scroll = t.comp.sel - height + 1
	}
}

func (m *Model) acceptCompletion(t *Tab, item lsp.CompletionItem) {
	if t == nil || t.ed.Buf.ReadOnly {
		return
	}
	text := item.InsertText
	r := ed.Range{Start: t.comp.start, End: t.comp.end}
	if item.TextEdit != nil {
		// textEdit авторитетнее insertText по спецификации LSP.
		text = item.TextEdit.NewText
		r = lspRangeToEd(t.ed.Buf, item.TextEdit.Range)
	}
	if text == "" {
		text = item.Label
	}
	m.closeOverlays()
	t.ed.ReplaceRange(r, text)
	m.afterEdit()
}

func (m *Model) completionView() popupView {
	t := m.tab()
	if t == nil || !t.comp.open {
		return popupView{}
	}
	inner := min(60, max(30, m.w-6))
	height := min(m.popupHeight(), max(1, len(t.comp.items)))
	start := max(0, min(t.comp.scroll, max(0, len(t.comp.items)-height)))
	if t.comp.sel < start {
		start = t.comp.sel
	}
	if t.comp.sel >= start+height {
		start = t.comp.sel - height + 1
	}
	t.comp.scroll = start

	rows := make([]string, 0, max(1, height))
	if len(t.comp.items) == 0 {
		rows = append(rows, m.th.panelDim+padANSI("  "+m.tr("нет совпадений", "no matches"), inner, "")+sgrReset)
	} else {
		for i := 0; i < height && start+i < len(t.comp.items); i++ {
			it := t.comp.items[start+i]
			mark := "  "
			style := m.th.panelOn
			if start+i == t.comp.sel {
				mark = "› "
				style = m.th.panelSel
			}
			right := strings.TrimSpace(it.Kind)
			if d := strings.TrimSpace(it.Detail); d != "" {
				if right != "" {
					right += "  "
				}
				right += d
			}
			right = sanitizeOneLine(right)
			left := sanitizeOneLine(it.Label)
			rightW := 0
			if right != "" {
				rightW = min(ansiWidth(right), max(0, inner/2))
				right = truncateTo(right, rightW)
			}
			leftW := max(0, inner-ansiWidth(mark)-rightW-2)
			line := mark + padANSI(truncateTo(left, leftW), leftW, "")
			if rightW > 0 {
				line += "  " + m.th.panelDim + right + sgrReset + style
			}
			rows = append(rows, style+padANSI(line, inner, "")+sgrReset)
		}
	}
	footer := m.trf("%d из %d · ↑↓ · Enter/Tab · Esc", "%d of %d · ↑↓ · Enter/Tab · Esc", len(t.comp.items), len(t.comp.all))
	return m.popupAtCursor(m.tr("Дополнение", "Completion"), footer, rows)
}

func (m *Model) completionClick(e tea.Mouse) tea.Cmd {
	v := m.completionView()
	if !v.shown {
		m.closeOverlays()
		return nil
	}
	if e.Y >= v.itemTop && e.Y < v.itemTop+v.itemCount {
		t := m.tab()
		idx := t.comp.scroll + (e.Y - v.itemTop)
		if t != nil && idx >= 0 && idx < len(t.comp.items) {
			m.acceptCompletion(t, t.comp.items[idx])
			return nil
		}
	}
	inside := e.X >= v.x && e.X < v.x+v.inner+2 && e.Y >= v.y && e.Y < v.y+len(v.box)
	if !inside {
		m.closeOverlays()
	}
	return nil
}

func (m *Model) completionHover(e tea.Mouse) {
	v := m.completionView()
	t := m.tab()
	if !v.shown || t == nil {
		return
	}
	if e.Y >= v.itemTop && e.Y < v.itemTop+v.itemCount {
		idx := t.comp.scroll + (e.Y - v.itemTop)
		if idx >= 0 && idx < len(t.comp.items) {
			t.comp.sel = idx
		}
	}
}

func (m *Model) completionWheel(up bool) {
	delta := 3
	if up {
		delta = -3
	}
	m.completionMove(delta)
}
