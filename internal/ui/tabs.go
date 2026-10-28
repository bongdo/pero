package ui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/bongdo/pero/internal/config"
	"github.com/bongdo/pero/internal/ed"
	"github.com/bongdo/pero/internal/workspace"
)

// isProsePath учитывает [files] из конфига: "*.md" = "code" переключает
// markdown-фичи и спеллчек для этих файлов.
func (m *Model) isProsePath(path string) bool {
	if path == "" {
		return true
	}
	if len(m.cfg.Files) > 0 {
		patterns := make([]string, 0, len(m.cfg.Files))
		for p := range m.cfg.Files {
			patterns = append(patterns, p)
		}
		sort.Strings(patterns)
		base := filepath.Base(path)
		for _, pattern := range patterns {
			ok := false
			if matched, err := filepath.Match(pattern, base); err == nil && matched {
				ok = true
			}
			if !ok {
				if matched, err := filepath.Match(pattern, path); err == nil && matched {
					ok = true
				}
			}
			if !ok {
				continue
			}
			switch strings.ToLower(strings.TrimSpace(m.cfg.Files[pattern])) {
			case "prose":
				return true
			case "code":
				return false
			}
		}
	}
	return workspace.IsProse(path) || strings.HasSuffix(strings.ToLower(path), ".markdown")
}

func (m *Model) tabNames() map[string]string {
	paths := make([]string, 0, len(m.tabs))
	for _, t := range m.tabs {
		if t.ed.Buf.Path != "" {
			paths = append(paths, t.ed.Buf.Path)
		}
	}
	return workspace.Disambiguate(paths)
}

func (m *Model) tabLabel(t *Tab) string {
	name := m.tr("без имени", "untitled")
	if t.ed.Buf.Path != "" {
		if n := m.tabNames()[t.ed.Buf.Path]; n != "" {
			name = n
		} else {
			name = workspace.DisplayName(t.ed.Buf.Path)
		}
	}
	if t.ed.Buf.Dirty() {
		name += " ●"
	}
	if t.deleted {
		name += " ×"
	}
	return sanitizeOneLine(name)
}

func (m *Model) newEmptyTab() {
	t := m.newTab(ed.New(""))
	m.tabs = append(m.tabs, t)
	m.active = len(m.tabs) - 1
	m.closeOverlays()
	m.scheduleGhost()
}

func (m *Model) newTabWithPalette() tea.Cmd {
	m.newEmptyTab()
	return m.openOpenPalette()
}

func (m *Model) switchTab(delta int) {
	if len(m.tabs) < 2 {
		return
	}
	if m.ov.kind == overlayCompletion {
		m.closeOverlays()
	}
	m.active = (m.active + delta + len(m.tabs)) % len(m.tabs)
	m.ov.menu = menuState{}
	if t := m.tab(); t != nil {
		t.free = false
	}
	m.checkExternalActive()
	m.scheduleGhost()
}

func (m *Model) switchTo(idx int) {
	if idx < 0 || idx >= len(m.tabs) || idx == m.active {
		return
	}
	if m.ov.kind == overlayCompletion {
		m.closeOverlays()
	}
	m.active = idx
	m.ov.menu = menuState{}
	if t := m.tab(); t != nil {
		t.free = false
	}
	m.checkExternalActive()
	m.scheduleGhost()
}

func (m *Model) checkExternalActive() {
	if t := m.tab(); t != nil {
		m.checkExternal(t, m.active, false)
	}
}

func (m *Model) reloadTab(t *Tab) error {
	cursor := t.ed.Cursor
	if err := t.ed.Buf.Reload(); err != nil {
		return err
	}
	m.detachLSP(t)
	ne := ed.NewEditor(t.ed.Buf)
	ne.TabWidth = m.cfg.Editor.Tabsize
	ne.Cursor = t.ed.Buf.Clamp(cursor)
	t.ed = ne
	t.roReason = ""
	if ne.Buf.ReadOnly {
		t.roReason = readOnlyReason(ne.Buf.Path)
	}
	t.wrapCache.valid = false
	t.hlVersion = -1
	t.diags = nil
	t.ghost = ghostState{}
	t.comp = completionState{}
	t.wordsVer = -1
	return nil
}

// checkExternal сравнивает mtime и не даёт молча перетереть чужое изменение.
func (m *Model) checkExternal(t *Tab, idx int, pendingSave bool) {
	path := t.ed.Buf.Path
	if path == "" {
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			t.deleted = true
		}
		return
	}
	t.deleted = false
	if t.mtime.IsZero() {
		t.mtime = info.ModTime()
		return
	}
	if info.ModTime().Equal(t.mtime) {
		return
	}
	// Не перетираем молча ни грязный, ни чистый буфер: пользователь выбирает.
	m.openExternalConfirm(idx, pendingSave)
}

func (m *Model) closeTabCmd(idx int) tea.Cmd {
	if idx < 0 || idx >= len(m.tabs) {
		return nil
	}
	t := m.tabs[idx]
	if t.ed.Buf.Dirty() {
		m.openConfirm(confirmCloseTab, idx, m.tr("Закрыть вкладку?", "Close tab?"),
			m.trf("%s содержит несохранённые изменения.", "%s has unsaved changes.", m.tabLabel(t)))
		return nil
	}
	m.closeTabNow(idx)
	return nil
}

func (m *Model) closeTabNow(idx int) {
	if idx < 0 || idx >= len(m.tabs) {
		return
	}
	if idx == m.active && m.ov.kind == overlayCompletion {
		m.closeOverlays()
	}
	m.detachLSP(m.tabs[idx])
	m.tabs = append(m.tabs[:idx], m.tabs[idx+1:]...)
	if len(m.tabs) == 0 {
		m.tabs = append(m.tabs, m.newTab(ed.New("")))
		m.active = 0
	} else if m.active > idx || m.active >= len(m.tabs) {
		m.active--
	}
	if m.active < 0 {
		m.active = 0
	}
	m.pendingClose = -1
	m.clampScroll()
	m.scheduleGhost()
}

func (m *Model) closeOthersCmd(keep int) tea.Cmd {
	if len(m.tabs) < 2 || keep < 0 || keep >= len(m.tabs) {
		return nil
	}
	dirty := false
	for i, t := range m.tabs {
		if i != keep && t.ed.Buf.Dirty() {
			dirty = true
			break
		}
	}
	if dirty {
		m.openConfirm(confirmCloseOthers, keep, m.tr("Закрыть остальные?", "Close other tabs?"),
			m.tr("Есть несохранённые вкладки. Сохранить их перед закрытием?",
				"Some tabs have unsaved changes. Save them before closing?"))
		return nil
	}
	m.closeOthersNow(keep)
	return nil
}

func (m *Model) closeOthersNow(keep int) {
	if keep < 0 || keep >= len(m.tabs) {
		return
	}
	if m.ov.kind == overlayCompletion {
		m.closeOverlays()
	}
	for i, t := range m.tabs {
		if i != keep {
			m.detachLSP(t)
		}
	}
	t := m.tabs[keep]
	m.tabs = []*Tab{t}
	m.active = 0
	m.clampScroll()
	m.scheduleGhost()
}

func (m *Model) saveCurrent() tea.Cmd {
	t := m.tab()
	if t == nil {
		return nil
	}
	if t.ed.Buf.ReadOnly {
		m.notifyError(m.tr("только чтение: сохранить нельзя", "read-only: cannot save"))
		return nil
	}
	if t.ed.Buf.Path == "" {
		m.openSaveAs()
		return nil
	}
	if m.externalChanged(t) {
		m.openExternalConfirm(m.active, true)
		return nil
	}
	m.doSave(t)
	return nil
}

func (m *Model) externalChanged(t *Tab) bool {
	path := t.ed.Buf.Path
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !t.mtime.IsZero() && !info.ModTime().Equal(t.mtime)
}

func (m *Model) doSave(t *Tab) bool {
	m.prepareSave(t, t.ed.Buf.Path)
	if err := t.ed.Buf.Save(); err != nil {
		m.notifyError(m.tr("не сохранено: ", "not saved: ") + err.Error())
		return false
	}
	t.mtime = fileMTime(t.ed.Buf.Path)
	t.deleted = false
	m.lspDidSave(t)
	m.rememberRecent(t.ed.Buf.Path)
	m.notifySuccess(m.tr("сохранено", "saved"))
	m.finishPendingClose()
	return true
}

// prepareSave применяет гигиену перед записью: хвостовые пробелы (только код)
// и завершающий перевод строки. Каждая правка - своя транзакция undo; если
// менять нечего, undo не растёт.
func (m *Model) prepareSave(t *Tab, path string) {
	if t == nil || t.ed.Buf.ReadOnly {
		return
	}
	changed := false
	if m.cfg.Editor.TrimTrailingWhitespace && !m.isProsePath(path) {
		changed = t.ed.TrimTrailingWhitespace() || changed
	}
	if m.cfg.Editor.FinalNewline {
		changed = t.ed.EnsureFinalNewline() || changed
	}
	if changed {
		m.afterEditTab(t)
	}
}

func (m *Model) finishPendingClose() {
	if m.pendingClose >= 0 {
		idx := m.pendingClose
		m.pendingClose = -1
		m.closeTabNow(idx)
	}
}

func (m *Model) openSaveAs() {
	t := m.tab()
	if t == nil {
		return
	}
	m.ov = overlay{kind: overlaySaveAs}
	m.ov.input.Set(t.ed.Buf.Path)
	m.ov.input.Placeholder = m.tr("путь к файлу", "file path")
}

func (m *Model) saveAs(path string) tea.Cmd {
	t := m.tab()
	if t == nil {
		return nil
	}
	// Неудачный Save As не должен оставлять pendingClose: иначе следующее
	// Ctrl-S в другой вкладке закроет эту без вопроса.
	fail := func(text string) tea.Cmd {
		m.pendingClose = -1
		m.notifyError(text)
		return nil
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return fail(m.tr("пустой путь", "empty path"))
	}
	path = expandHome(path)
	if !filepath.IsAbs(path) {
		base := m.root
		if base == "" {
			base, _ = os.Getwd()
		}
		path = filepath.Join(base, path)
	}
	path = filepath.Clean(path)
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return fail(m.tr("это папка: ", "this is a directory: ") + path)
	}
	if other := m.tabForPath(path); other != nil && other != t {
		return fail(m.tr("файл уже открыт в другой вкладке: ", "file is already open in another tab: ") + path)
	}
	m.detachLSP(t)
	m.prepareSave(t, path)
	if err := t.ed.Buf.SaveAs(path); err != nil {
		cmd := m.lspEnsure(t)
		m.pendingClose = -1
		m.notifyError(m.tr("не сохранено: ", "not saved: ") + err.Error())
		return cmd
	}
	t.mtime = fileMTime(path)
	t.deleted = false
	t.wrap = m.cfg.SoftwrapFor(path)
	// Пересобрать кэш подсветки под новый filetype.
	t.wrapCache.valid = false
	t.hlVersion = -1
	t.diags = nil
	t.diagVersion = 0
	t.ghost = ghostState{}
	m.rememberRecent(path)
	m.closeOverlays()
	m.notifySuccess(m.tr("сохранено: ", "saved: ") + path)
	m.clampScroll()
	if m.pendingClose >= 0 {
		m.finishPendingClose()
		return nil
	}
	return m.lspEnsure(t)
}

func (m *Model) rememberRecent(path string) {
	if path == "" {
		return
	}
	out := []string{path}
	for _, old := range m.recent {
		if old != path && len(out) < 20 {
			out = append(out, old)
		}
	}
	m.recent = out
}

func (m *Model) saveState() error {
	st := config.State{
		Recent:         append([]string(nil), m.recent...),
		RecentCommands: append([]string(nil), m.recentCmds...),
	}
	if t := m.tab(); t != nil {
		st.LastFile = t.ed.Buf.Path
	}
	if m.cfg.Editor.RestoreSession {
		for _, t := range m.tabs {
			if t.ed.Buf.Path == "" {
				continue
			}
			st.Tabs = append(st.Tabs, config.TabState{
				Path:       t.ed.Buf.Path,
				CursorLine: t.ed.Cursor.Line,
				CursorCol:  t.ed.Cursor.Col,
			})
		}
	}
	return config.SaveState(st)
}

func (m *Model) quitCmd() tea.Cmd {
	dirty := 0
	for _, t := range m.tabs {
		if t.ed.Buf.Dirty() {
			dirty++
		}
	}
	if dirty > 0 {
		m.openConfirm(confirmQuit, -1, m.tr("Выйти из pero?", "Quit pero?"),
			m.trf("Несохранённых вкладок: %d. Сохранить перед выходом?",
				"Unsaved tabs: %d. Save before quitting?", dirty))
		return nil
	}
	m.closeOverlays()
	_ = m.saveState()
	m.quitting = true
	m.closeAllLSP()
	return tea.Quit
}

// saveAll - для подтверждений; false, если что-то помешало (ничего не теряем).
func (m *Model) saveAll() bool {
	for _, t := range m.tabs {
		if !t.ed.Buf.Dirty() {
			continue
		}
		if t.ed.Buf.Path == "" {
			m.notifyError(m.tr("у вкладки нет пути: сохраните через Ctrl-Shift-S",
				"tab has no path: save it with Ctrl-Shift-S"))
			return false
		}
		if t.ed.Buf.ReadOnly {
			m.notifyError(m.tr("только чтение: ", "read-only: ") + t.ed.Buf.Path)
			return false
		}
		if m.externalChanged(t) {
			m.notifyError(m.tr("файл изменён извне: ", "file changed on disk: ") + filepath.Base(t.ed.Buf.Path))
			return false
		}
		m.prepareSave(t, t.ed.Buf.Path)
		if err := t.ed.Buf.Save(); err != nil {
			m.notifyError(m.tr("не сохранено: ", "not saved: ") + err.Error())
			return false
		}
		t.mtime = fileMTime(t.ed.Buf.Path)
		t.deleted = false
		m.lspDidSave(t)
		m.rememberRecent(t.ed.Buf.Path)
	}
	return true
}

// ---- полоса вкладок ----------------------------------------------------

type tabHit struct {
	idx, x, w int
}

type tabBarState struct {
	start, end int
	hits       []tabHit
	left       bool
	right      bool
}

func (m *Model) tabBarState() tabBarState {
	n := len(m.tabs)
	if n == 0 || m.w <= 0 {
		return tabBarState{}
	}
	labels := make([]string, n)
	widths := make([]int, n)
	maxLabel := max(6, min(20, m.w/2))
	for i, t := range m.tabs {
		l := " " + m.tabLabel(t) + " "
		l = truncateTo(l, maxLabel)
		labels[i] = l
		widths[i] = ansiWidth(l)
	}
	total := 0
	for _, w := range widths {
		total += w
	}
	start := 0
	if total > m.w {
		start = max(0, min(m.tabScroll, n-1))
		if m.active < start {
			start = m.active
		}
		for start < m.active {
			need := 2 // место под «‹»
			if start == 0 {
				need = 0
			}
			for i := start; i <= m.active; i++ {
				need += widths[i]
			}
			if need <= m.w || start == m.active {
				break
			}
			start++
		}
		m.tabScroll = start
	} else {
		m.tabScroll = 0
	}

	st := tabBarState{start: start, end: start}
	x := 0
	if start > 0 {
		st.left = true
		x += 2
	}
	for i := start; i < n; i++ {
		if x+widths[i] > m.w {
			break
		}
		st.hits = append(st.hits, tabHit{idx: i, x: x, w: widths[i]})
		x += widths[i]
		st.end = i + 1
	}
	if st.end < n {
		st.right = true
		// Освободить место под «› N», не выкидывая активную вкладку.
		for len(st.hits) > 0 {
			rightW := ansiWidth(" › " + itoa(n-st.end))
			if x+rightW <= m.w {
				break
			}
			last := st.hits[len(st.hits)-1]
			if last.idx == m.active {
				break
			}
			st.hits = st.hits[:len(st.hits)-1]
			st.end = last.idx
			x -= last.w
		}
	}
	return st
}

func (m *Model) tabRow() string {
	st := m.tabBarState()
	var b strings.Builder
	used := 0
	if st.left {
		b.WriteString(m.th.dim)
		b.WriteString("‹ ")
		b.WriteString(sgrReset)
		used += 2
	}
	for _, hit := range st.hits {
		label := truncateTo(" "+m.tabLabel(m.tabs[hit.idx])+" ", hit.w)
		if hit.idx == m.active {
			b.WriteString(m.th.tabActiveOn)
		} else {
			b.WriteString(m.th.tabInactiveOn)
		}
		b.WriteString(label)
		b.WriteString(sgrReset)
		used += ansiWidth(label)
	}
	if st.right {
		label := " › " + itoa(len(m.tabs)-st.end)
		b.WriteString(m.th.dim)
		b.WriteString(label)
		b.WriteString(sgrReset)
		used += ansiWidth(label)
	}
	if used < m.w {
		b.WriteString(m.th.tabBar)
		b.WriteString(strings.Repeat(" ", m.w-used))
		b.WriteString(sgrReset)
	}
	return b.String()
}

func (m *Model) tabAt(x, y int) (int, bool) {
	if y != 0 || len(m.tabs) < 2 {
		return 0, false
	}
	for _, h := range m.tabBarState().hits {
		if x >= h.x && x < h.x+h.w {
			return h.idx, true
		}
	}
	return 0, false
}

func (m *Model) scrollTabBar(delta int) {
	m.tabScroll = max(0, min(m.tabScroll+delta, max(0, len(m.tabs)-1)))
}

func (m *Model) tabBarScrollByWheel(up bool) {
	if up {
		m.scrollTabBar(-3)
	} else {
		m.scrollTabBar(3)
	}
}
