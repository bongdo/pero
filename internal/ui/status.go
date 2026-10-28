package ui

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bongdo/pero/internal/ed"
)

type statusSeg struct {
	text  string
	style string
}

func (m *Model) fileType(t *Tab) string {
	path := t.ed.Buf.Path
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	if ext == "" {
		return "txt"
	}
	return sanitizeOneLine(ext)
}

func (m *Model) eolName(t *Tab) string {
	if t.ed.Buf.EOL == "\r\n" {
		return "CRLF"
	}
	return "LF"
}

func (m *Model) statusSegments() ([]statusSeg, int, int) {
	t := m.tab()
	if t == nil {
		return nil, 0, 0
	}
	e, w, info := 0, 0, 0
	for _, d := range t.visibleDiags() {
		switch d.Severity {
		case 1:
			e++
		case 2:
			w++
		case 3, 4:
			info++
		}
	}
	segs := []statusSeg{
		{text: m.tabLabel(t), style: m.th.status + "\x1b[1m"},
	}
	compact := m.w < 60
	if !compact {
		segs = append(segs,
			statusSeg{text: m.fileType(t), style: m.th.status + m.th.muted},
			statusSeg{text: m.eolName(t), style: m.th.status + m.th.muted},
		)
	}
	segs = append(segs, statusSeg{
		text:  fmt.Sprintf("%d:%d", t.ed.Cursor.Line+1, t.ed.Cursor.Col+1),
		style: m.th.status,
	})
	if t.ed.HasSelection() {
		sel := t.ed.SelectionRange()
		chars := len([]rune(t.ed.SelectedText()))
		lines := sel.End.Line - sel.Start.Line + 1
		text := m.count(chars, "символ", "символа", "символов", "char", "chars")
		text += " / " + m.count(lines, "строка", "строки", "строк", "line", "lines")
		segs = append(segs, statusSeg{text: text, style: m.th.status + m.th.muted})
	}
	diagX, diagW := -1, 0
	if e > 0 || w > 0 || info > 0 {
		text := ""
		if e > 0 {
			text = m.count(e, "ошибка", "ошибки", "ошибок", "error", "errors")
		}
		if w > 0 {
			if text != "" {
				text += " "
			}
			text += m.count(w, "предупреждение", "предупреждения", "предупреждений", "warning", "warnings")
		}
		if info > 0 {
			if text != "" {
				text += " "
			}
			text += itoa(info) + " " + m.tr("инфо", "info")
		}
		style := m.th.status + m.th.info
		if e > 0 {
			style = m.th.status + m.th.error
		} else if w > 0 {
			style = m.th.status + m.th.warning
		}
		diagX = len(segs) // заполнится ниже реальной позицией
		diagW = ansiWidth(text)
		segs = append(segs, statusSeg{text: text, style: style})
	}
	if m.ov.kind != overlayHover {
		if d := m.diagAtCursor(t); d != nil && strings.TrimSpace(d.Message) != "" {
			limit := max(12, min(48, m.w/3))
			segs = append(segs, statusSeg{
				text:  truncateTo(sanitizeOneLine(d.Message), limit),
				style: m.th.status + m.th.muted,
			})
		}
	}
	if status := m.lspStatusText(t); status != "" {
		style := m.th.status + m.th.info
		if t.lspState == lspOff || t.lspState == lspFailed {
			style = m.th.status + m.th.muted
		}
		segs = append(segs, statusSeg{text: sanitizeOneLine(status), style: style})
	}
	if t.ed.Buf.ReadOnly {
		segs = append(segs, statusSeg{text: m.readOnlyLabel(t), style: m.th.status + m.th.muted})
	}
	if t.deleted {
		segs = append(segs, statusSeg{text: m.tr("удалён", "deleted"), style: m.th.status + m.th.error})
	}
	return segs, diagX, diagW
}

func (m *Model) statusLayout() (string, rect) {
	segs, diagIdx, diagW := m.statusSegments()
	var b strings.Builder
	b.WriteString(m.th.status)
	b.WriteString(" ")
	x := 1
	errRect := rect{x: 0, y: m.h - 1, w: 0, h: 1}
	diagX := -1
	for i, s := range segs {
		if i > 0 {
			b.WriteString("  ")
			x += 2
		}
		if i == diagIdx {
			diagX = x
		}
		b.WriteString(s.style)
		b.WriteString(s.text)
		b.WriteString(sgrReset)
		b.WriteString(m.th.status)
		x += ansiWidth(s.text)
	}
	if diagX >= 0 {
		errRect = rect{x: diagX, y: m.h - 1, w: diagW, h: 1}
	}

	hints := m.tr("F1 помощь   Ctrl-P открыть", "F1 help   Ctrl-P open")
	if m.w < 60 {
		hints = "F1 · ^P"
	}
	if x+2+ansiWidth(hints) <= m.w {
		pad := m.w - x - ansiWidth(hints) - 1
		b.WriteString(strings.Repeat(" ", max(1, pad)))
		b.WriteString(m.th.status + m.th.muted)
		b.WriteString(hints)
		b.WriteString(sgrReset)
		b.WriteString(m.th.status)
		x += max(1, pad) + ansiWidth(hints)
	}
	line := b.String()
	line = truncateTo(line, m.w)
	line = padANSI(line, m.w, m.th.status)
	return line, errRect
}

func (m *Model) readOnlyLabel(t *Tab) string {
	text := m.tr("только чтение", "read-only")
	switch t.roReason {
	case "binary":
		text += m.tr(" · бинарный", " · binary")
	case "large":
		text += m.tr(" · >10 МБ", " · >10 MB")
	}
	return text
}

// readOnlyReason повторяет критерии ed.Open: большой файл или NUL в начале.
func readOnlyReason(path string) string {
	if path == "" {
		return ""
	}
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	if info.Size() > ed.MaxFileSize {
		return "large"
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	buf := make([]byte, 8<<10)
	n, _ := io.ReadFull(f, buf)
	if bytes.IndexByte(buf[:n], 0) >= 0 {
		return "binary"
	}
	return ""
}

func pluralRu(n int, one, few, many string) string {
	if n%100 >= 11 && n%100 <= 14 {
		return many
	}
	switch n % 10 {
	case 1:
		return one
	case 2, 3, 4:
		return few
	}
	return many
}

func (m *Model) statusRow() string {
	line, _ := m.statusLayout()
	return line
}

func (m *Model) statusErrAt(x, y int) bool {
	if m.ov.kind != overlayNone || y != m.h-1 {
		return false
	}
	_, r := m.statusLayout()
	return r.w > 0 && x >= r.x && x < r.x+r.w
}
