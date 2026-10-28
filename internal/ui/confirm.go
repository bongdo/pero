package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/bongdo/pero/internal/widgets"
)

type confirmKind int

const (
	confirmCloseTab confirmKind = iota
	confirmCloseOthers
	confirmQuit
	confirmExternal
)

type confirmState struct {
	kind        confirmKind
	tab         int
	title       string
	text        string
	sel         int
	pendingSave bool
}

func (m *Model) openConfirm(kind confirmKind, tab int, title, text string) {
	if m.ov.cancel != nil {
		m.ov.cancel()
	}
	m.ov = overlay{kind: overlayConfirm}
	m.ov.confirm = confirmState{kind: kind, tab: tab, title: title, text: text}
}

func (m *Model) openExternalConfirm(tab int, pendingSave bool) {
	m.openConfirm(confirmExternal, tab, m.tr("Файл изменён на диске", "File changed on disk"),
		m.tr("Кто-то изменил файл. Перезагрузить с диска или оставить мои правки?",
			"The file was changed. Reload from disk or keep my changes?"))
	m.ov.confirm.pendingSave = pendingSave
	// Enter по умолчанию не разрушает правки.
	m.ov.confirm.sel = 1
}

func (m *Model) confirmButtons() []string {
	switch m.ov.confirm.kind {
	case confirmCloseTab:
		return []string{m.tr("Сохранить", "Save"), m.tr("Не сохранять", "Don't save"), m.tr("Отмена", "Cancel")}
	case confirmCloseOthers:
		return []string{m.tr("Сохранить всё", "Save all"), m.tr("Закрыть без сохранения", "Close without saving"), m.tr("Отмена", "Cancel")}
	case confirmQuit:
		return []string{m.tr("Сохранить всё", "Save all"), m.tr("Выйти без сохранения", "Quit without saving"), m.tr("Отмена", "Cancel")}
	case confirmExternal:
		return []string{m.tr("Перезагрузить", "Reload"), m.tr("Оставить моё", "Keep mine")}
	}
	return []string{m.tr("Отмена", "Cancel")}
}

func (m *Model) confirmClamp() {
	n := len(m.confirmButtons())
	m.ov.confirm.sel = (m.ov.confirm.sel%n + n) % n
}

func (m *Model) confirmKey(msg tea.KeyPressMsg) tea.Cmd {
	k := msg.Keystroke()
	switch k {
	case "esc":
		m.cancelConfirm()
		return nil
	case "left", "shift+tab", "h":
		m.ov.confirm.sel--
		m.confirmClamp()
		return nil
	case "right", "tab", "l":
		m.ov.confirm.sel++
		m.confirmClamp()
		return nil
	case "y", "Y":
		return m.activateConfirm(0)
	case "n", "N":
		return m.activateConfirm(1)
	case "enter", "space":
		return m.activateConfirm(m.ov.confirm.sel)
	}
	return nil
}

func (m *Model) cancelConfirm() {
	kind := m.ov.confirm.kind
	m.closeOverlays()
	if kind == confirmExternal || kind == confirmCloseTab {
		m.pendingClose = -1
	}
}

func (m *Model) activateConfirm(choice int) tea.Cmd {
	c := m.ov.confirm
	if choice < 0 || choice >= len(m.confirmButtons()) {
		return nil
	}
	// Внешнее изменение: 0 — перезагрузить, 1 — оставить моё.
	if c.kind == confirmExternal {
		return m.resolveExternal(choice == 0)
	}
	// Остальные: 0 — сохранить, 1 — без сохранения, 2 — отмена.
	switch choice {
	case 0:
		return m.confirmSave(c)
	case 1:
		return m.confirmDiscard(c)
	default:
		m.cancelConfirm()
		return nil
	}
}

func (m *Model) resolveExternal(reload bool) tea.Cmd {
	c := m.ov.confirm
	m.closeOverlays()
	if c.tab < 0 || c.tab >= len(m.tabs) {
		return nil
	}
	t := m.tabs[c.tab]
	if reload {
		if err := m.reloadTab(t); err != nil {
			m.notifyError(m.tr("не перезагружено: ", "not reloaded: ") + err.Error())
			m.pendingClose = -1
			return nil
		}
		t.mtime = fileMTime(t.ed.Buf.Path)
		t.deleted = false
		m.notifyInfo(m.tr("перезагружено с диска", "reloaded from disk"))
		m.finishPendingClose()
		return nil
	}
	t.mtime = fileMTime(t.ed.Buf.Path)
	t.deleted = false
	if c.pendingSave {
		m.doSave(t)
	}
	return nil
}

func (m *Model) confirmSave(c confirmState) tea.Cmd {
	switch c.kind {
	case confirmCloseTab:
		m.closeOverlays()
		if c.tab < 0 || c.tab >= len(m.tabs) {
			return nil
		}
		m.active = c.tab
		m.scheduleGhost()
		t := m.tabs[c.tab]
		if t.ed.Buf.ReadOnly {
			m.notifyError(m.tr("только чтение: закрыть без сохранения? Alt-Q", "read-only: close without saving? Alt-Q"))
			return nil
		}
		if t.ed.Buf.Path == "" {
			m.pendingClose = c.tab
			m.openSaveAs()
			return nil
		}
		m.pendingClose = c.tab
		if m.externalChanged(t) {
			m.openExternalConfirm(c.tab, true)
			return nil
		}
		if !m.doSave(t) {
			m.pendingClose = -1
		}
		return nil
	case confirmCloseOthers:
		m.closeOverlays()
		if !m.saveAll() {
			return nil
		}
		m.closeOthersNow(c.tab)
		m.notifySuccess(m.tr("остальные вкладки закрыты", "other tabs closed"))
		return nil
	case confirmQuit:
		m.closeOverlays()
		if !m.saveAll() {
			return nil
		}
		_ = m.saveState()
		m.quitting = true
		m.closeAllLSP()
		return tea.Quit
	}
	return nil
}

func (m *Model) confirmDiscard(c confirmState) tea.Cmd {
	switch c.kind {
	case confirmCloseTab:
		m.closeOverlays()
		m.closeTabNow(c.tab)
		return nil
	case confirmCloseOthers:
		m.closeOverlays()
		m.closeOthersNow(c.tab)
		return nil
	case confirmQuit:
		m.closeOverlays()
		_ = m.saveState()
		m.quitting = true
		m.closeAllLSP()
		return tea.Quit
	}
	return nil
}

type confirmView struct {
	shown   bool
	box     []string
	x, y    int
	buttons []rect
}

func (m *Model) confirmView() confirmView {
	if m.ov.kind != overlayConfirm {
		return confirmView{}
	}
	inner := max(10, min(max(36, m.w/2), m.w-4))
	lines := wrapWords(m.ov.confirm.text, max(8, inner-2))
	rows := make([]string, 0, len(lines)+2)
	for _, l := range lines {
		rows = append(rows, m.th.panelOn+l+sgrReset)
	}
	rows = append(rows, m.th.panel+strings.Repeat(" ", inner)+sgrReset)

	var (
		buttonRows [][]rect
		lineRects  []rect
		btnLine    strings.Builder
		used       int
	)
	btnLine.WriteString(m.th.panel)
	btnLine.WriteString("  ")
	used = 2
	for i, label := range m.confirmButtons() {
		text := "[ " + label + " ]"
		if used+ansiWidth(text) > inner && used > 2 {
			buttonRows = append(buttonRows, lineRects)
			lineRects = nil
			btnLine.WriteString(m.th.panel)
			btnLine.WriteString(strings.Repeat(" ", max(0, inner-used)))
			btnLine.WriteString(sgrReset)
			rows = append(rows, btnLine.String())
			btnLine.Reset()
			btnLine.WriteString(m.th.panel)
			btnLine.WriteString("  ")
			used = 2
		}
		if used+ansiWidth(text) > inner {
			break
		}
		if i == m.ov.confirm.sel {
			btnLine.WriteString(m.th.panelSel)
		} else {
			btnLine.WriteString(m.th.panelOn)
		}
		btnLine.WriteString(text)
		btnLine.WriteString(sgrReset)
		btnLine.WriteString(m.th.panel)
		btnLine.WriteString("  ")
		lineRects = append(lineRects, rect{x: used, w: ansiWidth(text)})
		used += ansiWidth(text) + 2
	}
	if used < inner {
		btnLine.WriteString(strings.Repeat(" ", inner-used))
	}
	buttonRows = append(buttonRows, lineRects)
	rows = append(rows, btnLine.String())

	box := widgets.RenderBox(widgets.Box{
		Title: m.ov.confirm.title, Rows: rows, Width: m.w,
		Footer: "y/n · ←→ Enter · Esc",
	}, m.th.widgetStyle())
	bw := 0
	for _, l := range box {
		bw = max(bw, ansiWidth(l))
	}
	x, y := widgets.Center(m.w, m.h, bw, len(box))
	var abs []rect
	for j, rects := range buttonRows {
		for _, r := range rects {
			abs = append(abs, rect{x: x + 1 + r.x, y: y + 1 + len(lines) + 1 + j, w: r.w, h: 1})
		}
	}
	return confirmView{shown: true, box: box, x: x, y: y, buttons: abs}
}
