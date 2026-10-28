package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/bongdo/pero/internal/ed"
)

func (m *Model) mouseClick(e tea.Mouse) tea.Cmd {
	m.drag.active = false
	if t := m.toastRect(); t.Contains(e.X, e.Y) && t.w > 0 {
		m.dismissToast()
		return nil
	}
	switch m.ov.kind {
	case overlayHelp:
		v := m.helpView()
		if len(v.box) > 0 {
			r := rect{x: v.x, y: v.y, w: ansiWidth(v.box[0]), h: len(v.box)}
			if !r.Contains(e.X, e.Y) {
				m.closeOverlays()
			}
		}
		return nil
	case overlayConfirm:
		v := m.confirmView()
		for i, r := range v.buttons {
			if r.Contains(e.X, e.Y) {
				return m.activateConfirm(i)
			}
		}
		return nil
	case overlayMenu:
		v := m.menuView()
		if !v.shown {
			m.closeMenu()
			return nil
		}
		r := rect{x: v.x, y: v.y, w: ansiWidth(v.box[0]), h: len(v.box)}
		if e.Button != tea.MouseLeft {
			// ПКМ по меню не должна активировать пункт.
			if !r.Contains(e.X, e.Y) {
				m.closeMenu()
			}
			return nil
		}
		for i, r := range v.rows {
			if r.w > 0 && r.Contains(e.X, e.Y) {
				return m.activateMenu(i)
			}
		}
		if !r.Contains(e.X, e.Y) {
			m.closeMenu()
		}
		return nil
	case overlayOpen:
		return m.openClick(e)
	case overlayCompletion:
		return m.completionClick(e)
	case overlayHover:
		return m.hoverClick(e)
	case overlayLocs:
		return m.locationsClick(e)
	case overlayQuickfix:
		return m.quickfixClick(e)
	case overlayAI:
		return m.aiClick(e)
	case overlayCommands:
		return m.commandPaletteClick(e)
	case overlayThemes:
		return m.themeClick(e)
	case overlaySaveAs:
		if e.Y != m.h-2 {
			m.pendingClose = -1
			m.closeOverlays()
		}
		return nil
	case overlayFind:
		lay := m.layout()
		if lay.find.Contains(e.X, e.Y) {
			row := e.Y - lay.find.y
			if m.ov.replace {
				m.ov.focus2 = row == 1
			} else {
				m.ov.focus2 = false
			}
		}
		return nil
	}

	if e.Button == tea.MouseMiddle {
		if idx, ok := m.tabAt(e.X, e.Y); ok {
			return m.closeTabCmd(idx)
		}
		return nil
	}
	if e.Button == tea.MouseRight {
		return m.mouseRight(e)
	}
	if e.Button != tea.MouseLeft {
		return nil
	}

	if e.Y == 0 && len(m.tabs) > 1 {
		st := m.tabBarState()
		if idx, ok := m.tabAt(e.X, e.Y); ok {
			m.switchTo(idx)
		} else if st.left && e.X < 2 {
			m.scrollTabBar(-3)
		} else if st.right && e.X >= m.w-2 {
			m.scrollTabBar(3)
		}
		return nil
	}
	if e.Y == m.h-1 && m.statusErrAt(e.X, e.Y) {
		m.jumpFirstDiag()
		return nil
	}

	pos, ok := m.posAt(e.X, e.Y)
	if !ok {
		return nil
	}
	t := m.tab()
	if t == nil {
		return nil
	}
	t.free = false

	now := time.Now()
	shift := e.Mod.Contains(tea.ModShift)
	samePos := m.click.pos == pos && now.Sub(m.click.when) < 450*time.Millisecond
	count := 1
	if samePos && shift == false && m.click.count > 0 {
		count = m.click.count + 1
		if count > 3 {
			count = 1
		}
	}
	m.click = clickState{when: now, count: count, pos: pos, tab: m.active}

	anchor := t.ed.Cursor
	if t.ed.HasSelection() {
		anchor = t.ed.Sel.Anchor
	}
	if shift && count == 1 {
		t.ed.SetSelection(anchor, pos)
		m.drag = dragState{active: true, anchor: anchor}
		m.scheduleGhost()
		return nil
	}
	t.ed.SetCursor(pos)
	m.drag = dragState{active: true, anchor: pos}
	switch count {
	case 2:
		t.ed.SetCursor(pos)
		t.ed.SelectWord()
	case 3:
		t.ed.SetCursor(pos)
		t.ed.SelectLine()
	}
	m.scheduleGhost()
	return nil
}

func (m *Model) mouseRelease(e tea.Mouse) tea.Cmd {
	if e.Button == tea.MouseLeft {
		m.drag.active = false
	}
	return nil
}

func (m *Model) mouseMotion(e tea.Mouse) tea.Cmd {
	switch m.ov.kind {
	case overlayHelp:
		return nil
	case overlayMenu:
		v := m.menuView()
		for i, r := range v.rows {
			if r.w > 0 && r.Contains(e.X, e.Y) && m.menuEnabled(i) && i != m.ov.menu.sel {
				m.ov.menu.sel = i
			}
		}
		return nil
	case overlayCommands:
		m.hoverCommandPalette(e)
		return nil
	case overlayThemes:
		m.themeHover(e)
		return nil
	case overlayOpen:
		m.hoverOpen(e)
		return nil
	case overlayCompletion:
		m.completionHover(e)
		return nil
	case overlayLocs:
		m.locationsHover(e)
		return nil
	case overlayQuickfix:
		m.quickfixHover(e)
		return nil
	case overlayFind, overlayConfirm, overlaySaveAs, overlayHover, overlayAI:
		return nil
	}
	if !m.drag.active {
		return nil
	}
	t := m.tab()
	if t == nil {
		return nil
	}
	lay := m.layout()
	// Автоскролл при выходе за текстовую зону.
	if e.Y < lay.text.y {
		t.top = max(0, t.top-1)
	} else if e.Y >= lay.text.y+lay.text.h {
		rows := t.screenRows(lay.text.w, m.cfg.Editor.Tabsize)
		t.top = min(max(0, len(rows)-max(1, lay.text.h)), t.top+1)
	}
	pos, ok := m.posAtClamped(e.X, e.Y)
	if !ok {
		return nil
	}
	t.ed.SetSelection(m.drag.anchor, pos)
	m.scheduleGhost()
	return nil
}

func (m *Model) mouseWheel(e tea.Mouse) tea.Cmd {
	up := e.Button == tea.MouseWheelUp
	delta := 3
	if up {
		delta = -3
	}
	switch m.ov.kind {
	case overlayHelp:
		m.ov.helpScroll += delta
		m.clampHelp()
		return nil
	case overlayMenu:
		m.closeMenu()
		return nil
	case overlayConfirm:
		return nil
	case overlayCommands:
		m.paletteMove(delta)
		return nil
	case overlayThemes:
		m.themeWheel(up)
		return nil
	case overlayOpen:
		m.openMove(delta)
		return nil
	case overlayCompletion:
		m.completionWheel(up)
		return nil
	case overlayLocs:
		m.locationsMove(delta)
		return nil
	case overlayQuickfix:
		m.quickfixMove(delta)
		return nil
	case overlayAI:
		m.aiWheel(up)
		return nil
	case overlaySaveAs, overlayFind, overlayHover:
		return nil
	}
	if e.Y == 0 && len(m.tabs) > 1 {
		m.tabBarScrollByWheel(up)
		return nil
	}
	t := m.tab()
	if t == nil {
		return nil
	}
	lay := m.layout()
	if lay.text.Contains(e.X, e.Y) {
		rows := t.screenRows(lay.text.w, m.cfg.Editor.Tabsize)
		t.free = true
		t.top = max(0, min(t.top+delta, max(0, len(rows)-max(1, lay.text.h))))
	}
	return nil
}

func (m *Model) mouseRight(e tea.Mouse) tea.Cmd {
	switch m.ov.kind {
	case overlayConfirm, overlayHelp:
		return nil
	default:
		if m.ov.kind != overlayNone {
			m.closeOverlays()
			return nil
		}
	}
	if e.Y == 0 && len(m.tabs) > 1 {
		if idx, ok := m.tabAt(e.X, e.Y); ok {
			m.openTabMenu(e.X, e.Y, idx)
		}
		return nil
	}
	pos, ok := m.posAt(e.X, e.Y)
	if !ok {
		return nil
	}
	t := m.tab()
	if t == nil {
		return nil
	}
	t.free = false
	if !t.ed.HasSelection() || !posInSelection(t, pos) {
		t.ed.SetCursor(pos)
		m.scheduleGhost()
	}
	m.openTextMenu(e.X, e.Y)
	return nil
}

func posInSelection(t *Tab, p ed.Pos) bool {
	if !t.ed.HasSelection() {
		return false
	}
	r := t.ed.SelectionRange()
	return posLessEq(r.Start, p) && posLess(p, r.End)
}

func (m *Model) posAt(x, y int) (ed.Pos, bool) {
	t := m.tab()
	if t == nil {
		return ed.Pos{}, false
	}
	lay := m.layout()
	if !lay.text.Contains(x, y) {
		return ed.Pos{}, false
	}
	rows := t.screenRows(lay.text.w, m.cfg.Editor.Tabsize)
	rowIdx := t.top + (y - lay.text.y)
	if rowIdx < 0 || rowIdx >= len(rows) {
		return ed.Pos{}, false
	}
	cell := x - lay.gutterW
	return posAtRow(t, rows, rowIdx, cell, t.left, m.cfg.Editor.Tabsize, t.wrap), true
}

func (m *Model) posAtClamped(x, y int) (ed.Pos, bool) {
	t := m.tab()
	if t == nil {
		return ed.Pos{}, false
	}
	lay := m.layout()
	if lay.text.w <= 0 || lay.text.h <= 0 {
		return ed.Pos{}, false
	}
	rows := t.screenRows(lay.text.w, m.cfg.Editor.Tabsize)
	if len(rows) == 0 {
		return ed.Pos{}, false
	}
	y = max(lay.text.y, min(y, lay.text.y+lay.text.h-1))
	x = max(lay.text.x, min(x, lay.text.x+lay.text.w-1))
	rowIdx := t.top + (y - lay.text.y)
	rowIdx = max(0, min(rowIdx, len(rows)-1))
	cell := x - lay.gutterW
	return posAtRow(t, rows, rowIdx, cell, t.left, m.cfg.Editor.Tabsize, t.wrap), true
}

func (m *Model) openClick(e tea.Mouse) tea.Cmd {
	v := m.openView()
	if !v.shown {
		return nil
	}
	if e.Y >= v.itemTop && e.Y < v.itemTop+v.itemCount {
		i := m.openStart() + (e.Y - v.itemTop)
		if i >= 0 && i < len(m.ov.matches) {
			m.ov.sel = i
			return m.chooseOpen()
		}
	}
	boxH := len(v.box)
	inside := e.X >= v.x && e.X < v.x+v.inner+2 && e.Y >= v.y && e.Y < v.y+boxH
	if !inside {
		m.closeOverlays()
	}
	return nil
}

func (m *Model) commandPaletteClick(e tea.Mouse) tea.Cmd {
	inputY, listY, listH := m.commandPaletteGeometry()
	if inputY < 0 {
		return nil
	}
	if e.Y == inputY {
		return nil
	}
	if e.Y >= listY && e.Y < listY+listH {
		idx := m.commandPaletteStart() + (e.Y - listY)
		items := m.paletteItems()
		if idx >= 0 && idx < len(items) {
			m.ov.sel = idx
			id := items[idx].id
			m.closeOverlays()
			return m.runCommand(id)
		}
		return nil
	}
	m.closeOverlays()
	return nil
}

func (m *Model) hoverCommandPalette(e tea.Mouse) {
	_, listY, listH := m.commandPaletteGeometry()
	if e.Y < listY || e.Y >= listY+listH {
		return
	}
	idx := m.commandPaletteStart() + (e.Y - listY)
	if idx >= 0 && idx < len(m.paletteItems()) {
		m.ov.sel = idx
	}
}

func (m *Model) hoverOpen(e tea.Mouse) {
	v := m.openView()
	if !v.shown {
		return
	}
	if e.Y >= v.itemTop && e.Y < v.itemTop+v.itemCount {
		i := m.openStart() + (e.Y - v.itemTop)
		if i >= 0 && i < len(m.ov.matches) {
			m.ov.sel = i
		}
	}
}

func (m *Model) jumpFirstDiag() {
	t := m.tab()
	if t == nil {
		return
	}
	diags := t.visibleDiags()
	if len(diags) == 0 {
		return
	}
	t.free = false
	first := 0
	for i := 1; i < len(diags); i++ {
		a := t.ed.Buf.PosFromUTF16(int(diags[i].Range.Start.Line), int(diags[i].Range.Start.Character))
		b := t.ed.Buf.PosFromUTF16(int(diags[first].Range.Start.Line), int(diags[first].Range.Start.Character))
		if posLess(a, b) {
			first = i
		}
	}
	p := t.ed.Buf.PosFromUTF16(int(diags[first].Range.Start.Line), int(diags[first].Range.Start.Character))
	t.ed.SetCursor(p)
	t.free = false
	m.ensureVisible(m.layout())
	m.clampScroll()
	m.scheduleGhost()
	if msg := diags[first].Message; msg != "" {
		m.notifyInfo(msg)
	}
}
