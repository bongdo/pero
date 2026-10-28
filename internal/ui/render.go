package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/bongdo/pero/internal/ed"
	"github.com/bongdo/pero/internal/highlight"
	"github.com/bongdo/pero/internal/widgets"
)

const sgrReset = "\x1b[0m"

func (m *Model) View() tea.View {
	m.w = max(1, m.w)
	m.h = max(1, m.h)
	lay := m.layout()
	m.ensureVisible(lay)
	m.updateBracketMatch()

	rows := make([]string, m.h)
	for i := range rows {
		rows[i] = m.blankRow()
	}
	if lay.tabs.h > 0 {
		rows[lay.tabs.y] = m.tabRow()
	}
	for i := 0; i < lay.text.h; i++ {
		rows[lay.text.y+i] = m.textRow(i, lay)
	}
	if lay.findH > 0 {
		fr := m.findRows()
		for i := 0; i < lay.findH && i < len(fr); i++ {
			rows[lay.find.y+i] = fr[i]
		}
	}
	rows[lay.status.y] = m.statusRow()

	rows = m.drawOverlays(rows)
	if line, x, y := m.toastView(); line != "" {
		rows = widgets.Overlay(rows, []string{line}, x, y, m.w)
	}

	for i := range rows {
		rows[i] = padANSI(truncateTo(rows[i], m.w), m.w, "")
		rows[i] += sgrReset
	}
	if len(rows) > m.h {
		rows = rows[:m.h]
	}
	for len(rows) < m.h {
		rows = append(rows, strings.Repeat(" ", m.w))
	}

	v := tea.NewView(strings.Join(rows, "\n"))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeAllMotion
	v.ReportFocus = true
	return v
}

func (m *Model) blankRow() string {
	return m.th.base + strings.Repeat(" ", m.w) + sgrReset
}

func (m *Model) drawOverlays(rows []string) []string {
	switch m.ov.kind {
	case overlayHelp:
		v := m.helpView()
		if len(v.box) > 0 {
			rows = widgets.Overlay(rows, v.box, v.x, v.y, m.w)
		}
	case overlayConfirm:
		v := m.confirmView()
		if len(v.box) > 0 {
			rows = widgets.Overlay(rows, v.box, v.x, v.y, m.w)
		}
	case overlayMenu:
		v := m.menuView()
		if v.shown && len(v.box) > 0 {
			rows = widgets.Overlay(rows, v.box, v.x, v.y, m.w)
		}
	case overlayOpen:
		v := m.openView()
		if v.shown && len(v.box) > 0 {
			rows = widgets.Overlay(rows, v.box, v.x, v.y, m.w)
		}
	case overlayCommands, overlaySaveAs:
		rows = m.drawInputOverlays(rows)
	case overlayThemes:
		rows = m.drawThemeOverlay(rows)
	case overlayCompletion:
		v := m.completionView()
		if v.shown && len(v.box) > 0 {
			rows = widgets.Overlay(rows, v.box, v.x, v.y, m.w)
		}
	case overlayHover:
		v := m.hoverView()
		if v.shown && len(v.box) > 0 {
			rows = widgets.Overlay(rows, v.box, v.x, v.y, m.w)
		}
	case overlayLocs:
		v := m.locationsView()
		if v.shown && len(v.box) > 0 {
			rows = widgets.Overlay(rows, v.box, v.x, v.y, m.w)
		}
	case overlayQuickfix:
		v := m.quickfixView()
		if v.shown && len(v.box) > 0 {
			rows = widgets.Overlay(rows, v.box, v.x, v.y, m.w)
		}
	case overlayAI:
		v := m.aiView()
		if v.shown && len(v.box) > 0 {
			rows = widgets.Overlay(rows, v.box, v.x, v.y, m.w)
		}
	}
	return rows
}

func (m *Model) ensureVisible(lay screenLayout) {
	t := m.tab()
	if t == nil || lay.text.h <= 0 || lay.text.w <= 0 {
		return
	}
	rows := t.screenRows(lay.text.w, m.cfg.Editor.Tabsize)
	cur := rowForPos(rows, t.wrapCache.lineStart, t.ed.Cursor)
	margin := m.cfg.Editor.ScrollMargin
	if margin*2 >= lay.text.h {
		margin = 0
	}
	if t.free {
		t.top = max(0, min(t.top, max(0, len(rows)-lay.text.h)))
		return
	}
	if cur < t.top+margin {
		t.top = cur - margin
	}
	if cur >= t.top+lay.text.h-margin {
		t.top = cur - lay.text.h + 1 + margin
	}
	t.top = max(0, min(t.top, max(0, len(rows)-lay.text.h)))
	if t.wrap {
		t.left = 0
		return
	}
	x := rowScreenCol(t, rows, cur, t.ed.Cursor.Col, m.cfg.Editor.Tabsize)
	if x < t.left {
		t.left = x
	}
	if x >= t.left+lay.text.w {
		t.left = x - lay.text.w + 1
	}
	t.left = max(0, t.left)
}

func (m *Model) textRow(i int, lay screenLayout) string {
	t := m.tab()
	if t == nil || lay.text.h <= 0 {
		return ""
	}
	rows := t.screenRows(lay.text.w, m.cfg.Editor.Tabsize)
	idx := t.top + i
	gutter := m.gutterRow(t, lay, rows, idx)
	if idx < 0 || idx >= len(rows) || lay.text.w <= 0 {
		return gutter + m.th.base + strings.Repeat(" ", max(0, lay.text.w)) + sgrReset
	}
	row := rows[idx]
	line := t.ed.Buf.Line(row.logical)
	clusters := lineClusters(line, m.cfg.Editor.Tabsize)
	spans := m.spansFor(t)
	var lineSpans []highlight.Span
	if row.logical < len(spans) {
		lineSpans = spans[row.logical]
	}
	diags := m.diagSpansFor(t, row.logical, len(line))
	cursorRow := rowForPos(rows, t.wrapCache.lineStart, t.ed.Cursor)
	cursorAtRowEnd := cursorRow == idx && t.ed.Cursor.Col == row.runeEnd
	cursorDrawn := false
	hasSel := t.ed.HasSelection()
	sel := t.ed.SelectionRange()
	curLine := t.ed.Cursor.Line

	// Ghost: только в активной вкладке и только когда не открыт оверлей.
	ghostText := ""
	ghostCol := t.ed.Cursor.Col
	if m.ov.kind == overlayNone && m.cfg.Editor.Ghost && t.ghost.text != "" && !hasSel &&
		cursorRow == idx && ghostCol >= row.runeStart && ghostCol <= row.runeEnd {
		ghostText = sanitizeOneLine(t.ghost.text)
	}
	ghostDrawn := false
	ghostShift := 0

	padStyle := m.th.base
	if row.logical == curLine {
		padStyle = m.th.current
	}

	var b strings.Builder
	used := 0
	curStyle := ""
	write := func(style, s string) {
		if s == "" {
			return
		}
		if style != curStyle {
			b.WriteString(sgrReset)
			b.WriteString(style)
			curStyle = style
		}
		b.WriteString(s)
	}
	pad := func(n int, style string) {
		if n <= 0 {
			return
		}
		write(style, strings.Repeat(" ", n))
		used += n
	}

	if row.indent > 0 {
		pad(min(row.indent, lay.text.w), padStyle)
	}

	si := 0
	ci := 0
	for ci < len(clusters) && clusters[ci].end <= row.runeStart {
		ci++
	}
	for ; ci < len(clusters) && clusters[ci].start < row.runeEnd; ci++ {
		cl := clusters[ci]
		// Ghost занимает место сразу после курсора и сдвигает остаток строки.
		if ghostText != "" && !ghostDrawn && cl.start >= ghostCol {
			if g := truncateTo(ghostText, lay.text.w-used); g != "" {
				write(m.th.ghostOn, g)
				ghostShift = ansiWidth(g)
				used += ghostShift
			}
			ghostDrawn = true
		}
		rw := cl.width
		x := row.indent + cl.col - row.startCol
		if !t.wrap {
			x = cl.col - t.left
		}
		x += ghostShift
		if x+rw > lay.text.w {
			break
		}
		if x > used {
			pad(x-used, padStyle)
		}
		if x < 0 {
			continue
		}
		i := cl.start
		for si < len(lineSpans) && lineSpans[si].End <= i {
			si++
		}
		spanSGR := ""
		if si < len(lineSpans) && lineSpans[si].Start <= i && i < lineSpans[si].End {
			spanSGR = lineSpans[si].SGR
		}
		p := ed.Pos{Line: row.logical, Col: cl.start}
		style := m.runeStyle(t, row.logical, p, spanSGR, hasSel, sel, diags, curLine)
		if cursorRow == idx && t.ed.Cursor.Col >= cl.start && t.ed.Cursor.Col < cl.end {
			style += m.th.cursor
			cursorDrawn = true
		} else if cursorAtRowEnd && cl.end == row.runeEnd && used+rw >= lay.text.w {
			// Строка заняла всю ширину: показываем курсор на последней ячейке,
			// иначе он окажется за экраном ровно на границе переноса.
			style += m.th.cursor
			cursorDrawn = true
		}
		ch := string(line[cl.start:cl.end])
		if cl.end-cl.start == 1 {
			switch r := line[cl.start]; {
			case r == '\t':
				ch = strings.Repeat(" ", max(1, rw))
			case r < 0x20 || r == 0x7f:
				ch = "�"
			}
		} else {
			ch = sanitizeOneLine(ch)
		}
		write(style, ch)
		used += rw
	}

	// Курсор в конце экранной строки.
	cursorHere := cursorRow == idx && t.ed.Cursor.Col >= row.runeEnd
	if cursorHere && !cursorDrawn && used < lay.text.w && t.ed.Cursor.Col <= len(line) {
		write(padStyle+m.th.cursor, " ")
		used++
	}
	// Ghost в конце строки (для середины слова он уже вставлен в цикле).
	if ghostText != "" && !ghostDrawn && cursorRow == idx && t.ed.Cursor.Col >= row.runeEnd && used < lay.text.w {
		g := truncateTo(ghostText, lay.text.w-used)
		write(m.th.ghostOn, g)
		used += ansiWidth(g)
	}
	// Перевод строки в выделении - одна ячейка после текста.
	if hasSel && !cursorHere && used < lay.text.w && idx == t.wrapCache.lineStart[row.logical+1]-1 &&
		posLess(ed.Pos{Line: row.logical, Col: len(line)}, sel.End) &&
		!posLess(ed.Pos{Line: row.logical, Col: len(line)}, sel.Start) {
		// ячейка «\n» входит в выделение
		write(m.th.selected, " ")
		used++
	}

	pad(lay.text.w-used, padStyle)
	b.WriteString(sgrReset)
	return gutter + b.String()
}

// updateBracketMatch пересчитывает пару под курсором для активной вкладки.
// Выделение сильнее подсветки скобок.
func (m *Model) updateBracketMatch() {
	t := m.tab()
	if t == nil {
		return
	}
	t.bracketOK = false
	e := t.ed
	if e.HasSelection() {
		return
	}
	line := e.Buf.Line(e.Cursor.Line)
	src := e.Cursor
	if src.Col >= len(line) || !isBracketRune(line[src.Col]) {
		if src.Col == 0 || !isBracketRune(line[src.Col-1]) {
			return
		}
		src.Col--
	}
	other, ok := e.Buf.MatchingBracket(e.Cursor)
	if !ok || other == src {
		return
	}
	t.bracketA, t.bracketB, t.bracketOK = src, other, true
}

func isBracketRune(r rune) bool {
	switch r {
	case '(', ')', '[', ']', '{', '}':
		return true
	}
	return false
}

func (m *Model) runeStyle(t *Tab, logical int, p ed.Pos, span string, hasSel bool, sel ed.Range, diags []diagSpan, curLine int) string {
	style := m.th.base
	if logical == curLine {
		style = m.th.current
	}
	searchKind := 0
	if m.ov.kind == overlayFind && m.ov.findLines != nil {
		for _, r := range m.ov.findLines[logical] {
			if posLessEq(r.Start, p) && posLess(p, r.End) {
				searchKind = 1
				if m.ov.cur >= 0 && m.ov.cur < len(m.ov.findMatches) && m.ov.findMatches[m.ov.cur] == r {
					searchKind = 2
				}
				break
			}
		}
	}
	switch searchKind {
	case 1:
		style = m.th.search
	case 2:
		style = m.th.searchCur
	}
	if hasSel && posLessEq(sel.Start, p) && posLess(p, sel.End) {
		style = m.th.selected
	}
	if span != "" && searchKind != 2 {
		style += span
	}
	if !hasSel && searchKind == 0 && t != nil && t.bracketOK &&
		(p == t.bracketA || p == t.bracketB) {
		style += m.th.bracketOn
	}
	if sev := diagAt(diags, p.Col); sev > 0 {
		current := logical == t.ed.Cursor.Line && p.Col == t.ed.Cursor.Col
		style += m.th.diagUnderline(sev, current)
	}
	return style
}

type diagSpan struct {
	start, end int
	sev        int
}

func (m *Model) diagSpansFor(t *Tab, logical, lineLen int) []diagSpan {
	diags := t.visibleDiags()
	if len(diags) == 0 {
		return nil
	}
	out := make([]diagSpan, 0, len(diags))
	for _, d := range diags {
		s := t.ed.Buf.PosFromUTF16(int(d.Range.Start.Line), int(d.Range.Start.Character))
		e := t.ed.Buf.PosFromUTF16(int(d.Range.End.Line), int(d.Range.End.Character))
		if s.Line > logical || e.Line < logical {
			continue
		}
		start := s.Col
		end := e.Col
		if s.Line < logical {
			start = 0
		}
		if e.Line > logical {
			end = lineLen
		}
		start = max(0, min(start, lineLen))
		end = max(0, min(end, lineLen))
		if start >= end {
			continue
		}
		out = append(out, diagSpan{start: start, end: end, sev: d.Severity})
	}
	return out
}

func diagAt(spans []diagSpan, col int) int {
	sev := 0
	for _, d := range spans {
		if col < d.start || col >= d.end {
			continue
		}
		s := d.sev
		if s < 1 || s > 3 {
			s = 3
		}
		if sev == 0 || s < sev {
			sev = s
		}
	}
	return sev
}

func (m *Model) gutterRow(t *Tab, lay screenLayout, rows []screenRow, idx int) string {
	if lay.gutterW <= 0 {
		return ""
	}
	digits := max(1, lay.gutterW-3)
	num := ""
	marker := " "
	style := m.th.gutterOn
	valid := idx >= 0 && idx < len(rows)
	first := false
	if valid {
		r := rows[idx]
		first = r.runeStart == 0
		if first {
			num = fmt.Sprint(r.logical + 1)
			if r.logical == t.ed.Cursor.Line {
				marker = "▏"
				style = m.th.accentOn + "\x1b[1m"
			}
		}
	}
	num = fmt.Sprintf("%*s", digits, num)
	return style + num + marker + "│ " + sgrReset
}

func (m *Model) spansFor(t *Tab) [][]highlight.Span {
	if t.hlVersion == t.ed.Buf.Version && t.hlLines != nil {
		return t.hlLines
	}
	// shortcut: на 20k+ строк chroma целиком на каждый ввод заметно тормозит;
	// подсветку включаем только для обычных файлов, вернуть при кэше по строкам.
	if len(t.ed.Buf.Lines) > 20000 {
		t.hlLines = nil
		t.hlVersion = t.ed.Buf.Version
		return nil
	}
	m.hl.SetFile(t.ed.Buf.Path)
	t.hlLines = m.hl.Lines(t.ed.Buf.Text())
	t.hlVersion = t.ed.Buf.Version
	return t.hlLines
}

// ---- ANSI-помощники ---------------------------------------------------

func ansiWidth(s string) int { return ansi.StringWidth(s) }

func truncateTo(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if ansi.StringWidth(s) <= w {
		return s
	}
	return ansi.Truncate(s, w, "")
}

func padANSI(s string, w int, style string) string {
	if w <= 0 {
		return ""
	}
	n := w - ansi.StringWidth(s)
	if n <= 0 {
		return s
	}
	if style == "" {
		return s + strings.Repeat(" ", n)
	}
	return s + style + strings.Repeat(" ", n) + sgrReset
}

// sanitizeOneLine убирает управляющие символы из имён файлов и сообщений:
// содержимое буфера и имена - недоверенные данные, ESC в терминал не пускаем.
func sanitizeOneLine(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\t' || r == '\n' || r == '\r':
			b.WriteByte(' ')
		case r < 0x20 || r == 0x7f:
			b.WriteRune('�')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func wrapWords(s string, width int) []string {
	if width <= 0 {
		return nil
	}
	indent := s[:len(s)-len(strings.TrimLeft(s, " "))]
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	var out []string
	cur := indent
	started := false
	for _, w := range words {
		if !started {
			cur += w
			started = true
			continue
		}
		if ansiWidth(cur)+1+ansiWidth(w) <= width {
			cur += " " + w
		} else {
			out = append(out, cur)
			cur = indent + w
		}
	}
	out = append(out, cur)
	return out
}
