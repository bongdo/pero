package ui

import (
	"context"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/bongdo/pero/internal/assist"
	"github.com/bongdo/pero/internal/ed"
	"github.com/bongdo/pero/internal/widgets"
)

const aiTimeout = 60 * time.Second

type aiMsg struct {
	tab     *Tab
	gen     int
	version int
	pos     ed.Pos
	rng     ed.Range
	replace bool
	text    string
	err     error
}

// aiSuggest — Alt-a: выделение или текущая строка уходит в opencode, результат
// ждёт в popup и в буфер сам не попадает.
func (m *Model) aiSuggest() tea.Cmd {
	t := m.tab()
	if t == nil {
		return nil
	}
	if m.ov.kind == overlayAI {
		m.notifyInfo(m.tr("AI: результат уже открыт", "AI: result is already open"))
		return nil
	}
	if t.ed.Buf.ReadOnly {
		m.notifyError(m.tr("только чтение: AI недоступен", "read-only: AI is unavailable"))
		return nil
	}

	pos := t.ed.Cursor
	rng := ed.Range{Start: pos, End: pos}
	replace := false
	text := string(t.ed.Buf.Line(pos.Line))
	if t.ed.HasSelection() {
		rng = t.ed.SelectionRange()
		replace = true
		text = t.ed.SelectedText()
	}
	if strings.TrimSpace(text) == "" {
		m.notifyInfo(m.tr("AI: выделите текст или встаньте на непустую строку",
			"AI: select text or move to a non-empty line"))
		return nil
	}

	if m.aiCancel != nil {
		m.aiCancel()
	}
	m.aiGen++
	gen := m.aiGen
	ctx, cancel := context.WithTimeout(context.Background(), aiTimeout)
	m.aiCancel = cancel

	version := t.ed.Buf.Version
	prompt := aiPrompt(t.ed.Buf.Path, m.fileType(t), assist.TruncateRunes(text, 60000))
	// Прогресс важнее старого тоста: явный Alt-a начинает новое действие.
	m.toastSeq++
	m.toast = widgets.Toast{Kind: "progress", Text: m.tr("AI думает… (Esc — отмена)", "AI is thinking… (Esc to cancel)")}
	return func() tea.Msg {
		out, err := assist.RunOpencode(ctx, prompt)
		return aiMsg{tab: t, gen: gen, version: version, pos: pos, rng: rng, replace: replace, text: out, err: err}
	}
}

func aiPrompt(path, filetype, text string) string {
	if strings.TrimSpace(path) == "" {
		path = "без имени"
	}
	return "Ты редактируешь текст в терминальном редакторе pero.\n" +
		"Улучши или продолжи фрагмент. Верни только готовый результат, без markdown-ограждений и пояснений.\n" +
		"Файл: " + path + " (" + filetype + ")\n\n" + text
}

func (m *Model) onAI(msg aiMsg) tea.Cmd {
	if msg.gen != m.aiGen {
		return nil
	}
	if m.aiCancel != nil {
		m.aiCancel() // освободить таймер контекста, результат уже пришёл
		m.aiCancel = nil
	}
	if m.toast.Kind == "progress" {
		m.toast = widgets.Toast{}
	}
	if !m.tabAlive(msg.tab) {
		return nil
	}
	if msg.err != nil {
		m.notifyError("AI: " + msg.err.Error())
		return nil
	}
	if msg.tab != m.tab() {
		m.notifyWarn(m.tr("AI: результат отброшен, вкладка не активна",
			"AI: result discarded, tab is not active"))
		return nil
	}
	if m.ov.kind != overlayNone {
		m.notifyWarn(m.tr("AI: результат отброшен, открыт другой диалог",
			"AI: result discarded, another dialog is open"))
		return nil
	}
	text := strings.TrimRight(msg.text, "\r\n")
	if strings.TrimSpace(text) == "" {
		m.notifyInfo(m.tr("AI: пустой ответ", "AI: empty response"))
		return nil
	}
	m.closeOverlays()
	m.ov.kind = overlayAI
	m.ov.aiText = text
	m.ov.aiTab = msg.tab
	m.ov.aiRange = msg.rng
	m.ov.aiPos = msg.pos
	m.ov.aiReplace = msg.replace
	m.ov.aiVersion = msg.version
	return nil
}

func (m *Model) aiKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.Keystroke() {
	case "esc":
		m.closeOverlays()
		return nil
	case "tab", "enter":
		return m.aiAccept()
	case "up", "k":
		m.ov.aiScroll--
	case "down", "j":
		m.ov.aiScroll++
	case "pgup":
		m.ov.aiScroll -= m.aiHeight()
	case "pgdown":
		m.ov.aiScroll += m.aiHeight()
	case "home":
		m.ov.aiScroll = 0
	case "end":
		m.ov.aiScroll = 1 << 20
	}
	m.aiClampScroll()
	return nil
}

// aiAccept вставляет результат одной транзакцией: вместо выделения или в
// сохранённую позицию курсора.
func (m *Model) aiAccept() tea.Cmd {
	t := m.ov.aiTab
	text := m.ov.aiText
	r := m.ov.aiRange
	pos := m.ov.aiPos
	replace := m.ov.aiReplace
	version := m.ov.aiVersion
	m.closeOverlays()

	if t == nil || !m.tabAlive(t) {
		return nil
	}
	if t.ed.Buf.ReadOnly {
		m.notifyError(m.tr("только чтение: вставка запрещена", "read-only: insert is disabled"))
		return nil
	}
	if t.ed.Buf.Version != version {
		m.notifyError(m.tr("AI: буфер изменился, результат не вставлен",
			"AI: buffer changed, result was not inserted"))
		return nil
	}
	if replace {
		t.ed.ReplaceRange(r, text)
	} else {
		t.ed.SetCursor(pos)
		t.ed.Insert(text)
	}
	m.afterEditTab(t)
	m.clampScroll()
	return nil
}

func (m *Model) aiHeight() int {
	return max(3, min(16, m.h-8))
}

func (m *Model) aiLines(width int) []string {
	var lines []string
	for _, raw := range strings.Split(m.ov.aiText, "\n") {
		raw = strings.TrimRight(raw, "\r")
		if strings.TrimSpace(raw) == "" {
			lines = append(lines, "")
			continue
		}
		lines = append(lines, wrapWords(raw, width)...)
	}
	return lines
}

func (m *Model) aiClampScroll() {
	lines := m.aiLines(min(76, max(24, m.w-8)))
	maxScroll := max(0, len(lines)-m.aiHeight())
	m.ov.aiScroll = max(0, min(m.ov.aiScroll, maxScroll))
}

func (m *Model) aiView() popupView {
	if m.ov.kind != overlayAI || strings.TrimSpace(m.ov.aiText) == "" {
		return popupView{}
	}
	inner := min(76, max(24, m.w-8))
	lines := m.aiLines(inner)
	height := m.aiHeight()
	maxScroll := max(0, len(lines)-height)
	start := max(0, min(m.ov.aiScroll, maxScroll))
	rows := make([]string, 0, height)
	for i := 0; i < height; i++ {
		text := ""
		if start+i < len(lines) {
			text = sanitizeOneLine(lines[start+i])
		}
		rows = append(rows, m.th.panelOn+padANSI(truncateTo(text, inner), inner, "")+sgrReset)
	}
	return m.popupAtCursor("AI", m.tr("Tab вставить · Esc закрыть", "Tab insert · Esc close"), rows)
}

func (m *Model) aiClick(e tea.Mouse) tea.Cmd {
	v := m.aiView()
	if !v.shown {
		m.closeOverlays()
		return nil
	}
	inside := e.X >= v.x && e.X < v.x+v.inner+2 && e.Y >= v.y && e.Y < v.y+len(v.box)
	if !inside {
		m.closeOverlays()
	}
	return nil
}

func (m *Model) aiWheel(up bool) {
	if up {
		m.ov.aiScroll -= 3
	} else {
		m.ov.aiScroll += 3
	}
	m.aiClampScroll()
}
