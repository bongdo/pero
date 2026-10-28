package ui

import (
	"fmt"
)

// rect - полуоткрытый прямоугольник в ячейках терминала.
type rect struct{ x, y, w, h int }

func (r rect) Contains(x, y int) bool {
	return r.w > 0 && r.h > 0 && x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h
}

func (r rect) Empty() bool { return r.w <= 0 || r.h <= 0 }

type screenLayout struct {
	w, h   int
	tabs   rect
	gutter rect
	text   rect
	find   rect
	status rect

	gutterW int
	findH   int
}

// layout - зоны экрана. Оверлеи, кроме строки поиска, не меняют зоны: они
// рисуются поверх готовых строк.
func (m *Model) layout() screenLayout {
	l := screenLayout{w: max(1, m.w), h: max(1, m.h)}
	l.status = rect{0, l.h - 1, l.w, 1}

	y := 0
	if len(m.tabs) > 1 && l.h >= 3 {
		l.tabs = rect{0, 0, l.w, 1}
		y = 1
	}

	if m.ov.kind == overlayFind {
		l.findH = 1
		if m.ov.replace {
			l.findH = 2
		}
		if room := l.h - y - 1; l.findH > room {
			l.findH = max(0, room)
		}
	}

	textH := l.h - y - 1 - l.findH
	if textH < 0 {
		textH = 0
	}
	l.find = rect{0, y + textH, l.w, l.findH}
	l.text = rect{0, y, l.w, textH}

	if l.w >= 40 && textH > 0 {
		last := 1
		if t := m.tab(); t != nil {
			last = t.ed.Buf.LastLine() + 1
		}
		digits := len(fmt.Sprint(last))
		l.gutterW = digits + 3
		if l.gutterW > l.w-1 {
			l.gutterW = 0
		}
	}
	l.gutter = rect{0, l.text.y, l.gutterW, l.text.h}
	l.text.x = l.gutterW
	l.text.w = max(0, l.w-l.gutterW)
	return l
}

// pageHeight - высота текстовой зоны для PgUp/PgDn.
func (m *Model) pageHeight() int {
	if h := m.layout().text.h; h > 1 {
		return h
	}
	return 1
}
