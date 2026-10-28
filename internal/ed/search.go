package ed

import (
	"sort"
	"strings"
	"unicode"
)

// FindAll ищет literal-вхождения без пересечений. Пустой запрос - ничего.
func (e *Editor) FindAll(query string, foldCase bool) []Range {
	if query == "" {
		return nil
	}
	q := []rune(query)
	if strings.ContainsRune(query, '\n') {
		return e.findAllMultiline(q, foldCase)
	}
	var out []Range
	for line, text := range e.Buf.Lines {
		for i := 0; i+len(q) <= len(text); {
			if runesMatch(text[i:i+len(q)], q, foldCase) {
				out = append(out, Range{Start: Pos{line, i}, End: Pos{line, i + len(q)}})
				i += len(q)
			} else {
				i++
			}
		}
	}
	return out
}

func (e *Editor) findAllMultiline(q []rune, foldCase bool) []Range {
	text := []rune(e.Buf.Text())
	if len(q) > len(text) {
		return nil
	}
	starts := make([]int, len(e.Buf.Lines))
	off := 0
	for i, line := range e.Buf.Lines {
		starts[i] = off
		off += len(line) + 1
	}
	posAt := func(off int) Pos {
		i := sort.Search(len(starts), func(i int) bool { return starts[i] > off }) - 1
		if i < 0 {
			i = 0
		}
		return Pos{i, off - starts[i]}
	}
	var out []Range
	for i := 0; i+len(q) <= len(text); {
		if runesMatch(text[i:i+len(q)], q, foldCase) {
			out = append(out, Range{Start: posAt(i), End: posAt(i + len(q))})
			i += len(q)
		} else {
			i++
		}
	}
	return out
}

// FindNext - первое совпадение начиная с from, по кругу.
func (e *Editor) FindNext(query string, from Pos, foldCase bool) (Range, bool) {
	matches := e.FindAll(query, foldCase)
	if len(matches) == 0 {
		return Range{}, false
	}
	from = e.Buf.Clamp(from)
	for _, r := range matches {
		if !posLess(r.Start, from) {
			return r, true
		}
	}
	return matches[0], true
}

// FindPrev - последнее совпадение до from, по кругу.
func (e *Editor) FindPrev(query string, from Pos, foldCase bool) (Range, bool) {
	matches := e.FindAll(query, foldCase)
	if len(matches) == 0 {
		return Range{}, false
	}
	from = e.Buf.Clamp(from)
	for i := len(matches) - 1; i >= 0; i-- {
		if posLess(matches[i].Start, from) {
			return matches[i], true
		}
	}
	return matches[len(matches)-1], true
}

func runesMatch(text, q []rune, foldCase bool) bool {
	if len(text) != len(q) {
		return false
	}
	for i := range q {
		if text[i] == q[i] {
			continue
		}
		if !foldCase || !runeEqualFold(text[i], q[i]) {
			return false
		}
	}
	return true
}

func runeEqualFold(a, b rune) bool {
	if a == b {
		return true
	}
	r := unicode.SimpleFold(a)
	for i := 0; i < 8 && r != a; i++ {
		if r == b {
			return true
		}
		r = unicode.SimpleFold(r)
	}
	return false
}
