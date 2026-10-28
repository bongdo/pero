package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/bongdo/pero/internal/ed"
	"github.com/bongdo/pero/internal/lsp"
)

type lspActionsMsg struct {
	tab     *Tab
	gen     int
	version int
	line    int
	actions []lsp.CodeAction
	err     error
}

type lspResolveMsg struct {
	tab    *Tab
	gen    int
	action lsp.CodeAction
	err    error
}

// quickfixCmd собирает диагностики текущей строки и спрашивает codeAction с
// диапазоном всей строки — так делают ltex и большинство серверов.
func (m *Model) quickfixCmd() tea.Cmd {
	t := m.tab()
	if t == nil {
		return nil
	}
	if t.ed.Buf.ReadOnly {
		m.notifyError(m.tr("только чтение: исправления запрещены", "read-only: quick fixes are disabled"))
		return nil
	}
	if !m.lspReadyFor(t) {
		m.lspOffToast(t)
		return nil
	}
	if !t.lsp.Supports("codeAction") {
		m.notifyInfo(m.tr("LSP не поддерживает исправления", "LSP does not support code actions"))
		return nil
	}
	line := t.ed.Cursor.Line
	lineLen := len(t.ed.Buf.Line(line))
	rng := lsp.Range{
		Start: lsp.Position{Line: line, Character: 0},
		End: lsp.Position{
			Line:      line,
			Character: t.ed.Buf.UTF16Offset(ed.Pos{Line: line, Col: lineLen}),
		},
	}
	diags := diagsOnLine(t, line)

	m.fixGen++
	gen := m.fixGen
	client, uri := t.lsp, t.lspURI
	version := t.ed.Buf.Version
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		actions, err := client.CodeActions(ctx, uri, rng, diags)
		return lspActionsMsg{tab: t, gen: gen, version: version, line: line, actions: actions, err: err}
	}
}

func (m *Model) onActions(msg lspActionsMsg) tea.Cmd {
	t := msg.tab
	if !m.tabAlive(t) || t != m.tab() || msg.gen != m.fixGen || t.ed.Buf.Version != msg.version {
		return nil
	}
	if m.ov.kind != overlayNone && m.ov.kind != overlayQuickfix {
		return nil
	}
	if msg.err != nil {
		if !m.failLSP(t, msg.err) {
			m.notifyError(m.tr("исправление: ", "code action: ") + m.lspErrorText(msg.err))
		}
		return nil
	}
	actions := make([]lsp.CodeAction, 0, len(msg.actions))
	for _, a := range msg.actions {
		if strings.TrimSpace(a.Title) != "" {
			actions = append(actions, a)
		}
	}
	if len(actions) == 0 {
		m.notifyInfo(m.tr("нет фиксов", "no fixes"))
		return nil
	}
	if len(actions) == 1 {
		return m.applyCodeAction(t, actions[0])
	}
	m.closeOverlays()
	m.ov.kind = overlayQuickfix
	m.ov.acts = actions
	m.ov.sel = 0
	m.ov.actLine = msg.line
	if ds := diagsOnLine(t, msg.line); len(ds) > 0 && strings.TrimSpace(ds[0].Message) != "" {
		m.ov.actTitle = ds[0].Message
	} else {
		m.ov.actTitle = m.tr("Исправить", "Quick fix")
	}
	return nil
}

func (m *Model) quickfixKey(msg tea.KeyPressMsg) tea.Cmd {
	t := m.tab()
	if t == nil {
		m.closeOverlays()
		return nil
	}
	k := msg.Keystroke()
	switch k {
	case "esc":
		m.closeOverlays()
		return nil
	case "up", "ctrl+p":
		m.quickfixMove(-1)
		return nil
	case "down", "ctrl+n":
		m.quickfixMove(1)
		return nil
	case "enter":
		if m.ov.sel >= 0 && m.ov.sel < len(m.ov.acts) {
			return m.applyCodeAction(t, m.ov.acts[m.ov.sel])
		}
		return nil
	}
	if len(msg.Text) == 1 && msg.Text >= "1" && msg.Text <= "9" &&
		!msg.Mod.Contains(tea.ModCtrl) && !msg.Mod.Contains(tea.ModAlt) {
		idx := int(msg.Text[0] - '1')
		if idx < len(m.ov.acts) {
			return m.applyCodeAction(t, m.ov.acts[idx])
		}
	}
	return nil
}

func (m *Model) quickfixMove(delta int) {
	n := len(m.ov.acts)
	if n == 0 {
		return
	}
	m.ov.sel = max(0, min(m.ov.sel+delta, n-1))
}

// applyCodeAction применяет Edit или выполняет серверную команду. Команда
// может привести к workspace/applyEdit — его обрабатывает lspApplyMsg.
func (m *Model) applyCodeAction(t *Tab, action lsp.CodeAction) tea.Cmd {
	m.closeOverlays()
	if action.Edit != nil {
		return m.applyWorkspaceEdit(action.Edit, action.Title)
	}
	if action.Command != nil {
		if t == nil || !m.lspReadyFor(t) {
			m.notifyInfo(m.tr("сервер вернул команду, LSP не запущен", "server returned a command, LSP is not running"))
			return nil
		}
		client := t.lsp
		cmd := action.Command
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()
			err := client.ExecuteCommand(ctx, cmd.Command, cmd.Arguments)
			return lspCommandMsg{title: action.Title, err: err}
		}
	}
	if len(action.Data) == 0 || !m.lspReadyFor(t) {
		m.notifyInfo(m.tr("нет фиксов", "no fixes"))
		return nil
	}
	m.resolveGen++
	gen := m.resolveGen
	client := t.lsp
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		resolved, err := client.ResolveCodeAction(ctx, action)
		return lspResolveMsg{tab: t, gen: gen, action: resolved, err: err}
	}
}

type lspCommandMsg struct {
	title string
	err   error
}

func (m *Model) onResolve(msg lspResolveMsg) tea.Cmd {
	if !m.tabAlive(msg.tab) || msg.tab != m.tab() || msg.gen != m.resolveGen {
		return nil
	}
	if m.ov.kind != overlayNone && m.ov.kind != overlayQuickfix {
		return nil
	}
	if msg.err != nil {
		if !m.failLSP(msg.tab, msg.err) {
			m.notifyError(m.tr("исправление: ", "code action: ") + m.lspErrorText(msg.err))
		}
		return nil
	}
	if msg.action.Edit == nil {
		if msg.action.Command != nil {
			m.notifyInfo(m.tr("сервер вернул команду: ", "server returned a command: ") + msg.action.Title)
		} else {
			m.notifyInfo(m.tr("нет фиксов", "no fixes"))
		}
		return nil
	}
	return m.applyWorkspaceEdit(msg.action.Edit, msg.action.Title)
}

// applyWorkspaceEdit правит все затронутые файлы; на каждый файл — одна
// транзакция. Файлы, которых нет среди вкладок, открываются.
func (m *Model) applyWorkspaceEdit(edit *lsp.WorkspaceEdit, title string) tea.Cmd {
	if edit == nil || len(edit.Changes) == 0 {
		m.notifyInfo(m.tr("нет фиксов", "no fixes"))
		return nil
	}
	uris := make([]string, 0, len(edit.Changes))
	for uri := range edit.Changes {
		uris = append(uris, uri)
	}
	sort.Strings(uris)

	changed := 0
	failed := 0
	for _, uri := range uris {
		edits := edit.Changes[uri]
		if len(edits) == 0 {
			continue
		}
		path, err := lsp.URIToPath(uri)
		if err != nil {
			failed++
			continue
		}
		t := m.tabForPath(path)
		if t == nil {
			m.openFile(path)
			t = m.tabForPath(path)
		}
		if t == nil {
			failed++
			continue
		}
		if t.ed.Buf.ReadOnly {
			m.notifyError(m.tr("только чтение: ", "read-only: ") + filepath.Base(path))
			failed++
			continue
		}
		if applyTextEdits(t.ed, edits) == 0 {
			continue
		}
		t.deleted = false
		t.ghost = ghostState{}
		m.lspDidChange(t)
		changed++
	}
	if changed == 0 {
		if failed > 0 {
			m.notifyError(m.tr("правка не применена", "change was not applied"))
		} else {
			m.notifyInfo(m.tr("нет фиксов", "no fixes"))
		}
		return nil
	}
	if failed > 0 {
		m.notifyWarn(m.trf("исправлено частично (%d), ошибок: %d",
			"partially fixed (%d), errors: %d", changed, failed))
		return nil
	}
	m.clampScroll()
	m.notifySuccess(m.tr("исправлено: ", "fixed: ") + title)
	return nil
}

func (m *Model) quickfixStart() int {
	height := min(8, max(1, len(m.ov.acts)))
	start := max(0, min(m.ov.sel-height+1, len(m.ov.acts)-height))
	if m.ov.sel < height {
		start = 0
	}
	return start
}

func (m *Model) quickfixView() popupView {
	if len(m.ov.acts) == 0 {
		return popupView{}
	}
	title := truncateTo(sanitizeOneLine(m.ov.actTitle), min(60, max(20, m.w-8)))
	height := min(8, len(m.ov.acts))
	start := m.quickfixStart()
	rows := make([]string, 0, height)
	inner := min(64, max(30, m.w-6))
	for i := 0; i < height; i++ {
		idx := start + i
		a := m.ov.acts[idx]
		label := sanitizeOneLine(a.Title)
		if k := strings.TrimSpace(a.Kind); k != "" {
			label += "  " + k
		}
		label = sanitizeOneLine(label)
		line := fmt.Sprintf("%2d  %s", idx+1, label)
		style := m.th.panelOn
		if idx == m.ov.sel {
			style = m.th.panelSel
			line = "› " + strings.TrimPrefix(line, " ")
		}
		rows = append(rows, style+padANSI(truncateTo(line, inner), inner, "")+sgrReset)
	}
	footer := fmt.Sprintf("%d–%d · ↑↓ · ↵ · Esc", start+1, start+height)
	return m.popupAtCursor(title, footer, rows)
}

func (m *Model) quickfixClick(e tea.Mouse) tea.Cmd {
	v := m.quickfixView()
	t := m.tab()
	if !v.shown || t == nil {
		m.closeOverlays()
		return nil
	}
	if e.Y >= v.itemTop && e.Y < v.itemTop+v.itemCount {
		idx := m.quickfixStart() + (e.Y - v.itemTop)
		if idx >= 0 && idx < len(m.ov.acts) {
			return m.applyCodeAction(t, m.ov.acts[idx])
		}
	}
	inside := e.X >= v.x && e.X < v.x+v.inner+2 && e.Y >= v.y && e.Y < v.y+len(v.box)
	if !inside {
		m.closeOverlays()
	}
	return nil
}

func (m *Model) quickfixHover(e tea.Mouse) {
	v := m.quickfixView()
	if !v.shown {
		return
	}
	if e.Y >= v.itemTop && e.Y < v.itemTop+v.itemCount {
		idx := m.quickfixStart() + (e.Y - v.itemTop)
		if idx >= 0 && idx < len(m.ov.acts) {
			m.ov.sel = idx
		}
	}
}

func (m *Model) locationsClick(e tea.Mouse) tea.Cmd {
	v := m.locationsView()
	if !v.shown {
		m.closeOverlays()
		return nil
	}
	if e.Y >= v.itemTop && e.Y < v.itemTop+v.itemCount {
		idx := m.locationsStart() + (e.Y - v.itemTop)
		if idx >= 0 && idx < len(m.ov.locs) {
			return m.gotoLocation(m.ov.locs[idx])
		}
	}
	inside := e.X >= v.x && e.X < v.x+v.inner+2 && e.Y >= v.y && e.Y < v.y+len(v.box)
	if !inside {
		m.closeOverlays()
	}
	return nil
}

func (m *Model) locationsHover(e tea.Mouse) {
	v := m.locationsView()
	if !v.shown || len(m.ov.locs) == 0 {
		return
	}
	if e.Y >= v.itemTop && e.Y < v.itemTop+v.itemCount {
		idx := m.locationsStart() + (e.Y - v.itemTop)
		if idx >= 0 && idx < len(m.ov.locs) {
			m.ov.sel = idx
		}
	}
}

func (m *Model) hoverClick(e tea.Mouse) tea.Cmd {
	v := m.hoverView()
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
