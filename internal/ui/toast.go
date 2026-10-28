package ui

import (
	"time"

	"github.com/bongdo/pero/internal/widgets"
)

func (m *Model) toastView() (string, int, int) {
	if m.toast.Text == "" {
		return "", 0, 0
	}
	line := widgets.RenderToast(m.toast, m.w, m.th.widgetStyle())
	if line == "" || !m.toast.Until.IsZero() && !m.toast.Until.After(time.Now()) {
		return "", 0, 0
	}
	w := ansiWidth(line)
	x := max(0, m.w-w-1)
	y := m.h - 2
	switch m.ov.kind {
	case overlayFind:
		if lay := m.layout(); lay.findH > 0 {
			y = lay.find.y - 1
		}
	case overlayCommands, overlaySaveAs:
		y = m.h - 3
	case overlayThemes:
		y = m.h - 4
	}
	if y < 0 {
		return "", 0, 0
	}
	// Не перекрывать курсор: если он на этой строке — поднять тост выше.
	if t := m.tab(); t != nil {
		lay := m.layout()
		rows := t.screenRows(lay.text.w, m.cfg.Editor.Tabsize)
		curRow := rowForPos(rows, t.wrapCache.lineStart, t.ed.Cursor)
		curY := lay.text.y + curRow - t.top
		if curY == y {
			y--
		}
	}
	if y < 0 {
		y = 0
	}
	return line, x, y
}

func (m *Model) toastRect() rect {
	line, x, y := m.toastView()
	if line == "" {
		return rect{}
	}
	return rect{x: x, y: y, w: ansiWidth(line), h: 1}
}

func (m *Model) dismissToast() {
	if m.toast.Kind == "progress" {
		return
	}
	m.toast = widgets.Toast{}
}
