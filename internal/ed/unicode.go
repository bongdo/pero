package ed

import "unicode"

// Правила склейки графемных кластеров - упрощённый UAX #29.
// Позиции в буфере остаются по рунам, но курсор и удаление двигаются по
// кластерам, чтобы не разрывать эмодзи, флаги и составные буквы.

func isRegionalIndicator(r rune) bool { return r >= 0x1F1E6 && r <= 0x1F1FF }

func isGraphemeExtend(r rune) bool {
	switch {
	case r == 0x200C, r == 0x200D: // ZWNJ, ZWJ
		return true
	case r >= 0xFE00 && r <= 0xFE0F: // variation selectors
		return true
	case r >= 0xE0100 && r <= 0xE01EF:
		return true
	case r >= 0x1F3FB && r <= 0x1F3FF: // skin tones
		return true
	case r >= 0x1F9B0 && r <= 0x1F9B3: // hair components
		return true
	case r >= 0xE0020 && r <= 0xE007F: // tag characters
		return true
	}
	return unicode.In(r, unicode.Mn, unicode.Mc, unicode.Me)
}

func isHangulL(r rune) bool { return (r >= 0x1100 && r <= 0x115F) || (r >= 0xA960 && r <= 0xA97C) }
func isHangulV(r rune) bool { return (r >= 0x1160 && r <= 0x11A7) || (r >= 0xD7B0 && r <= 0xD7C6) }
func isHangulT(r rune) bool { return (r >= 0x11A8 && r <= 0x11FF) || (r >= 0xD7CB && r <= 0xD7FB) }

func isHangulLV(r rune) bool  { return r >= 0xAC00 && r <= 0xD7A3 && (r-0xAC00)%28 == 0 }
func isHangulLVT(r rune) bool { return r >= 0xAC00 && r <= 0xD7A3 && (r-0xAC00)%28 != 0 }

func hangulJoin(prev, cur rune) bool {
	if isHangulL(prev) {
		return isHangulL(cur) || isHangulV(cur) || isHangulLV(cur) || isHangulLVT(cur)
	}
	if isHangulLV(prev) || isHangulV(prev) {
		return isHangulV(cur) || isHangulT(cur)
	}
	if isHangulLVT(prev) || isHangulT(prev) {
		return isHangulT(cur)
	}
	return false
}

// graphemeBreak сообщает, можно ли разорвать строку перед руной i.
func graphemeBreak(line []rune, i int) bool {
	if i <= 0 || i >= len(line) {
		return true
	}
	prev, cur := line[i-1], line[i]
	if prev == '\r' && cur == '\n' {
		return false
	}
	if isGraphemeExtend(cur) {
		return false
	}
	if prev == 0x200D { // упрощение GB11: после ZWJ не рвём
		return false
	}
	if isRegionalIndicator(prev) && isRegionalIndicator(cur) {
		n := 0
		for j := i - 1; j >= 0 && isRegionalIndicator(line[j]); j-- {
			n++
		}
		return n%2 == 0
	}
	return !hangulJoin(prev, cur)
}

// clusterEnd - индекс после кластера, начинающегося в start.
func clusterEnd(line []rune, start int) int {
	if start < 0 {
		start = 0
	}
	if start >= len(line) {
		return len(line)
	}
	i := start + 1
	for i < len(line) && !graphemeBreak(line, i) {
		i++
	}
	return i
}

// clusterStart - индекс начала кластера, заканчивающегося в end.
func clusterStart(line []rune, end int) int {
	if end > len(line) {
		end = len(line)
	}
	if end <= 0 {
		return 0
	}
	i := end - 1
	for i > 0 && !graphemeBreak(line, i) {
		i--
	}
	return i
}

// snapClusterStart прижимает позицию внутрь кластера к его началу.
func snapClusterStart(line []rune, col int) int {
	if col <= 0 || col >= len(line) {
		return col
	}
	for col > 0 && !graphemeBreak(line, col) {
		col--
	}
	return col
}

// snapClusterEnd прижимает позицию внутрь кластера к его концу.
func snapClusterEnd(line []rune, col int) int {
	if col <= 0 || col >= len(line) {
		return col
	}
	if graphemeBreak(line, col) {
		return col
	}
	return clusterEnd(line, snapClusterStart(line, col))
}
