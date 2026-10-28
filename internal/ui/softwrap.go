package ui

import (
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
	"github.com/rivo/uniseg"

	"github.com/bongdo/pero/internal/ed"
)

// screenRow — одна экранная строка: кусок логической строки плюс отступ,
// который рисуется перед содержимым при переносе.
type screenRow struct {
	logical            int
	runeStart, runeEnd int
	startCol           int // экранная колонка runeStart в логической строке
	indent             int // ячеек отступа у продолжения
}

// cluster — графемный кластер: единица отрисовки и ширины.
type cluster struct {
	start, end int // индексы рун [start,end)
	col        int // экранная колонка начала
	width      int
	space      bool
}

func lineClusters(line []rune, tabsize int) []cluster {
	if tabsize < 1 {
		tabsize = 4
	}
	if len(line) == 0 {
		return nil
	}
	// Быстрый путь для латиницы/кириллицы: каждая руна — отдельный кластер.
	// uniseg нужен только для комбинирующих/эмодзи/табов.
	if simpleLine(line) {
		out := make([]cluster, 0, len(line))
		col := 0
		for i, r := range line {
			w := runewidth.RuneWidth(r)
			if w < 0 {
				w = 0
			}
			out = append(out, cluster{start: i, end: i + 1, col: col, width: w, space: r == ' '})
			col += w
		}
		return out
	}
	g := uniseg.NewGraphemes(string(line))
	out := make([]cluster, 0, len(line))
	runeOff, col := 0, 0
	for g.Next() {
		s := g.Str()
		n := utf8.RuneCountInString(s)
		r := []rune(s)
		w := 0
		space := false
		switch {
		case n == 1 && r[0] == '\t':
			w = tabsize - col%tabsize
			space = true
		case n == 1 && (r[0] < 0x20 || r[0] == 0x7f):
			w = 1
		default:
			w = ansi.StringWidth(s)
			if n == 1 && r[0] == ' ' {
				space = true
			}
		}
		if w < 0 {
			w = 0
		}
		out = append(out, cluster{start: runeOff, end: runeOff + n, col: col, width: w, space: space})
		runeOff += n
		col += w
	}
	return out
}

// simpleLine — руны без комбинирования, ZWJ, эмодзи, CJK, табов и
// variation selectors. Для таких строк ширина руны = runewidth.
func simpleLine(line []rune) bool {
	for _, r := range line {
		switch {
		case r == '\t', r == 0x200D, r == 0xFE0F:
			return false
		case r >= 0x1F000: // эмодзи, regional indicators
			return false
		case r >= 0x1100 && r <= 0x11FF: // Hangul Jamo
			return false
		case r >= 0x2E80 && r <= 0xA4CF: // CJK и производные
			return false
		case r >= 0xAC00 && r <= 0xD7AF: // Hangul syllables
			return false
		case r >= 0xFE00 && r <= 0xFE0F:
			return false
		case r >= 0xE0100 && r <= 0xE01EF:
			return false
		case unicode.Is(unicode.Mn, r), unicode.Is(unicode.Me, r):
			return false
		}
	}
	return true
}

type wrapKey struct {
	width int
	tab   int
	wrap  bool
}

type wrapCache struct {
	key       wrapKey
	valid     bool
	rows      []screenRow
	lineStart []int
	lineRows  [][]screenRow
	lineHash  []uint64
}

// screenRows возвращает карту экранных строк вкладки. Перенос считается по
// видимой ширине; при правке пересчитываются только изменившиеся логические
// строки (полный пересчёт 100k строк иначе занимает сотни миллисекунд).
func (t *Tab) screenRows(width, tabsize int) []screenRow {
	if tabsize < 1 {
		tabsize = 4
	}
	width = max(1, width)
	key := wrapKey{width: width, tab: tabsize, wrap: t.wrap}
	lines := t.ed.Buf.Lines
	if !t.wrapCache.valid || t.wrapCache.key != key || len(t.wrapCache.lineHash) != len(lines) {
		t.wrapCache = wrapCache{
			key:      key,
			valid:    true,
			lineRows: make([][]screenRow, len(lines)),
			lineHash: make([]uint64, len(lines)),
		}
		for i, line := range lines {
			t.wrapCache.lineRows[i] = t.wrapLogicalLine(line, i, width, tabsize)
			t.wrapCache.lineHash[i] = hashRunes(line)
		}
		t.flattenScreenRows()
		return t.wrapCache.rows
	}

	changed := false
	for i, line := range lines {
		h := hashRunes(line)
		if h == t.wrapCache.lineHash[i] {
			continue
		}
		t.wrapCache.lineHash[i] = h
		t.wrapCache.lineRows[i] = t.wrapLogicalLine(line, i, width, tabsize)
		changed = true
	}
	if !changed {
		return t.wrapCache.rows
	}
	t.flattenScreenRows()
	return t.wrapCache.rows
}

func (t *Tab) wrapLogicalLine(line []rune, logical, width, tabsize int) []screenRow {
	if t.wrap {
		return wrapLine(line, logical, width, tabsize)
	}
	return []screenRow{{logical: logical, runeEnd: len(line)}}
}

func (t *Tab) flattenScreenRows() {
	total := 0
	for _, rows := range t.wrapCache.lineRows {
		total += len(rows)
	}
	rows := make([]screenRow, 0, total)
	lineStart := make([]int, len(t.wrapCache.lineRows)+1)
	for i, lr := range t.wrapCache.lineRows {
		lineStart[i] = len(rows)
		rows = append(rows, lr...)
	}
	lineStart[len(t.wrapCache.lineRows)] = len(rows)
	t.wrapCache.rows = rows
	t.wrapCache.lineStart = lineStart
}

func hashRunes(line []rune) uint64 {
	const (
		offset = 14695981039346656037
		prime  = 1099511628211
	)
	h := uint64(offset)
	for _, r := range line {
		h ^= uint64(r)
		h *= prime
	}
	return h
}

// rowForPos находит экранную строку курсора. Позиция ровно на границе
// переноса остаётся в конце предыдущей экранной строки.
func rowForPos(rows []screenRow, lineStart []int, p ed.Pos) int {
	if len(rows) == 0 {
		return 0
	}
	line := p.Line
	if line < 0 {
		line = 0
	}
	if line >= len(lineStart)-1 {
		line = len(lineStart) - 2
	}
	lo, hi := lineStart[line], lineStart[line+1]
	if lo >= hi {
		return lo
	}
	for i := lo; i < hi; i++ {
		if p.Col <= rows[i].runeEnd {
			return i
		}
	}
	return hi - 1
}

// rowScreenCol — экранная колонка логической позиции внутри её строки
// (без горизонтального скролла).
func rowScreenCol(t *Tab, rows []screenRow, rowIdx int, col, tabsize int) int {
	if rowIdx < 0 || rowIdx >= len(rows) {
		return 0
	}
	r := rows[rowIdx]
	line := t.ed.Buf.Line(r.logical)
	col = max(r.runeStart, min(col, r.runeEnd))
	x := r.indent + screenColOf(line, col, tabsize) - screenColOf(line, r.runeStart, tabsize)
	if x < 0 {
		x = 0
	}
	return x
}

// screenColOf — экранная колонка начала кластера, содержащего col.
func screenColOf(line []rune, col, tabsize int) int {
	col = max(0, min(col, len(line)))
	total := 0
	for _, cl := range lineClusters(line, tabsize) {
		if col < cl.end {
			return cl.col
		}
		if col == cl.end {
			// граница: если это не конец строки, следующая колонка та же
			total = cl.col + cl.width
			continue
		}
		total = cl.col + cl.width
	}
	return total
}

// posAtRow — обратное к rowScreenCol: экранная ячейка → логическая позиция.
// cell уже без учёта горизонтального сдвига; left применяется только без wrap.
func posAtRow(t *Tab, rows []screenRow, rowIdx, cell, left, tabsize int, wrap bool) ed.Pos {
	if rowIdx < 0 || rowIdx >= len(rows) {
		return ed.Pos{}
	}
	r := rows[rowIdx]
	line := t.ed.Buf.Line(r.logical)
	target := cell
	if wrap {
		if target <= r.indent {
			return ed.Pos{Line: r.logical, Col: r.runeStart}
		}
		target = r.startCol + target - r.indent
	} else {
		target += left
	}
	for _, cl := range lineClusters(line, tabsize) {
		if cl.end <= r.runeStart {
			continue
		}
		if cl.start >= r.runeEnd {
			break
		}
		if target < cl.col+cl.width {
			return ed.Pos{Line: r.logical, Col: cl.start}
		}
	}
	return ed.Pos{Line: r.logical, Col: r.runeEnd}
}

// displayCols — экранная колонка каждой руны (tabs раскрываются, эмодзи
// считаются кластером).
func displayCols(line []rune, tabsize int) []int {
	cols := make([]int, len(line)+1)
	for _, cl := range lineClusters(line, tabsize) {
		for i := cl.start; i < cl.end; i++ {
			cols[i] = cl.col
		}
		cols[cl.end] = cl.col + cl.width
	}
	if len(line) > 0 && cols[len(line)] == 0 {
		cols[len(line)] = cols[len(line)-1]
	}
	return cols
}

// wrapLine режет логическую строку на экранные по словам; продолжения
// сохраняют ведущий отступ. Слово шире экрана режется по кластерам.
func wrapLine(line []rune, logical, width, tabsize int) []screenRow {
	width = max(1, width)
	clusters := lineClusters(line, tabsize)
	if len(clusters) == 0 {
		return []screenRow{{logical: logical}}
	}

	indentRunes := 0
	for indentRunes < len(line) && (line[indentRunes] == ' ' || line[indentRunes] == '\t') {
		indentRunes++
	}
	cont := screenColOf(line, indentRunes, tabsize)
	if cont > width/2 {
		cont = width / 2
	}
	if cont >= width {
		cont = 0
	}

	var out []screenRow
	start := 0
	first := true
	for {
		avail := width
		ind := 0
		if !first {
			ind = cont
			avail = max(1, width-cont)
		}
		used := 0
		end := start
		lastSpace := -1
		for end < len(clusters) {
			cl := clusters[end]
			if used+cl.width > avail {
				if end == start {
					end++ // кластер шире строки — рисуем как есть
				}
				break
			}
			if cl.space {
				lastSpace = end
			}
			used += cl.width
			end++
		}
		if end < len(clusters) && lastSpace >= start && lastSpace+1 > start {
			end = lastSpace + 1
		}
		startRune := len(line)
		startCol := screenColOf(line, len(line), tabsize)
		if start < len(clusters) {
			startRune = clusters[start].start
			startCol = clusters[start].col
		}
		endRune := len(line)
		if end < len(clusters) {
			endRune = clusters[end].start
		}
		out = append(out, screenRow{
			logical: logical, runeStart: startRune, runeEnd: endRune,
			startCol: startCol, indent: ind,
		})
		if end >= len(clusters) {
			break
		}
		start = end
		first = false
	}
	return out
}

func posLess(a, b ed.Pos) bool {
	return a.Line < b.Line || (a.Line == b.Line && a.Col < b.Col)
}

// screenHeight — высота текстовой зоны; PgUp/PgDn двигают на неё, а не на
// число логических строк (важно для softwrap).
func (m *Model) screenHeight() int {
	if lay := m.layout(); lay.text.h > 0 {
		return lay.text.h
	}
	return 1
}

// moveScreen двигает курсор на delta экранных строк, сохраняя экранную
// колонку. В softwrap это переходит между перенесёнными частями строки.
func (m *Model) moveScreen(delta int, extend bool) {
	t := m.tab()
	if t == nil || delta == 0 {
		return
	}
	lay := m.layout()
	width := max(1, lay.text.w)
	rows := t.screenRows(width, m.cfg.Editor.Tabsize)
	if len(rows) == 0 {
		return
	}
	lineStart := t.wrapCache.lineStart
	curRow := rowForPos(rows, lineStart, t.ed.Cursor)
	// rowForPos держит курсор на границе в конце предыдущей строки. Для
	// движения вниз/вверх это значит, что первый шаг должен перейти границу.
	if delta > 0 && curRow+1 < len(rows) && rows[curRow].runeEnd == t.ed.Cursor.Col &&
		rows[curRow+1].logical == rows[curRow].logical && rows[curRow+1].runeStart == t.ed.Cursor.Col {
		curRow++
	} else if delta < 0 && curRow+1 < len(rows) && rows[curRow].runeEnd == t.ed.Cursor.Col &&
		rows[curRow+1].logical == rows[curRow].logical && rows[curRow+1].runeStart == t.ed.Cursor.Col {
		curRow++
	} else if delta < 0 && curRow > 0 && rows[curRow].runeStart == t.ed.Cursor.Col &&
		rows[curRow-1].logical == rows[curRow].logical && rows[curRow-1].runeEnd == t.ed.Cursor.Col {
		curRow--
	}
	cell := rowScreenCol(t, rows, curRow, t.ed.Cursor.Col, m.cfg.Editor.Tabsize)
	if !t.wrap {
		cell = max(0, cell-t.left)
	}
	target := max(0, min(curRow+delta, len(rows)-1))
	pos := posAtRow(t, rows, target, cell, t.left, m.cfg.Editor.Tabsize, t.wrap)
	if extend {
		anchor := t.ed.Cursor
		if t.ed.Sel.Active {
			anchor = t.ed.Sel.Anchor
		}
		t.ed.SetSelection(anchor, pos)
	} else {
		t.ed.SetCursor(pos)
	}
	t.free = false
	m.clampScroll()
}

func posLessEq(a, b ed.Pos) bool { return !posLess(b, a) }
