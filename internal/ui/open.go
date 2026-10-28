package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/bongdo/pero/internal/ed"
	"github.com/bongdo/pero/internal/widgets"
	"github.com/bongdo/pero/internal/workspace"
)

const openListLimit = 12

type openFilesMsg struct {
	gen   int
	files []workspace.File
	done  bool
}

type openDoneMsg struct{ gen int }

func (m *Model) walkRoot() string {
	root := m.root
	if root == "" {
		root, _ = os.Getwd()
	}
	root = expandHome(root)
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	return root
}

func (m *Model) openOpenPalette() tea.Cmd {
	m.closeOverlays()
	m.ov.kind = overlayOpen
	m.ov.input.Placeholder = m.tr("файл", "file")
	m.openGen++
	m.ov.gen = m.openGen
	m.ov.loading = true
	m.ov.files = nil
	m.ov.matches = nil
	m.ov.total = 0
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan workspace.File, 256)
	m.ov.ch = ch
	m.ov.cancel = cancel
	go workspace.Walk(ctx, m.walkRoot(), m.cfg.Ignore.Dirs, ch)
	m.recomputeOpenMatches()
	return m.readOpenChunk(m.ov.gen, ch)
}

func (m *Model) readOpenChunk(gen int, ch <-chan workspace.File) tea.Cmd {
	return func() tea.Msg {
		var batch []workspace.File
		f, ok := <-ch
		if !ok {
			return openDoneMsg{gen: gen}
		}
		batch = append(batch, f)
		for len(batch) < 200 {
			select {
			case f, ok := <-ch:
				if !ok {
					return openFilesMsg{gen: gen, files: batch, done: true}
				}
				batch = append(batch, f)
			default:
				return openFilesMsg{gen: gen, files: batch}
			}
		}
		return openFilesMsg{gen: gen, files: batch}
	}
}

func (m *Model) onOpenFiles(msg openFilesMsg) tea.Cmd {
	if msg.gen != m.ov.gen || m.ov.kind != overlayOpen {
		return nil
	}
	m.ov.files = append(m.ov.files, msg.files...)
	m.ov.total = len(m.ov.files)
	if msg.done {
		m.ov.loading = false
	} else {
		m.ov.loading = true
	}
	m.recomputeOpenMatches()
	if msg.done {
		return nil
	}
	return m.readOpenChunk(msg.gen, m.ov.ch)
}

// pathMode — ввод похож на путь, а не на fuzzy-фильтр.
func pathMode(q string) bool {
	return strings.HasPrefix(q, "~") || strings.HasPrefix(q, "/") || strings.HasPrefix(q, "./") || strings.HasPrefix(q, "../")
}

func (m *Model) recomputeOpenMatches() {
	q := strings.TrimSpace(m.ov.input.Value())
	m.ov.matches = nil
	if pathMode(q) {
		m.ov.matches = m.dirMatches(q)
		m.ov.total = len(m.ov.matches)
		m.ov.sel = max(0, min(m.ov.sel, max(0, len(m.ov.matches)-1)))
		return
	}
	if q == "" {
		seen := map[string]bool{}
		for _, rp := range m.recent {
			if len(m.ov.matches) >= openListLimit {
				break
			}
			info, err := os.Stat(rp)
			if err != nil || info.IsDir() {
				continue
			}
			if seen[rp] {
				continue
			}
			seen[rp] = true
			rel := rp
			if r, err := filepath.Rel(m.walkRoot(), rp); err == nil && !strings.HasPrefix(r, "..") {
				rel = r
			}
			m.ov.matches = append(m.ov.matches, workspace.Match{
				File: workspace.File{Path: rp, Rel: rel, Size: info.Size(), ModTime: info.ModTime()},
			})
		}
	}
	got := workspace.Filter(m.ov.files, q, 0)
	for _, mt := range got {
		if len(m.ov.matches) >= 200 {
			break
		}
		dup := false
		for _, have := range m.ov.matches {
			if have.File.Path == mt.File.Path {
				dup = true
				break
			}
		}
		if !dup {
			m.ov.matches = append(m.ov.matches, mt)
		}
	}
	m.ov.total = len(m.ov.files)
	m.ov.sel = max(0, min(m.ov.sel, max(0, len(m.ov.matches)-1)))
}

func (m *Model) dirMatches(q string) []workspace.Match {
	expanded := expandHome(q)
	if strings.HasSuffix(q, "/") || q == "~" {
		expanded = strings.TrimSuffix(expanded, "/")
	}
	dir := filepath.Dir(expanded)
	base := filepath.Base(expanded)
	if strings.HasSuffix(q, "/") || q == "~" {
		dir = expanded
		base = ""
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(m.walkRoot(), dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []workspace.Match
	lower := strings.ToLower(base)
	for _, ent := range entries {
		name := ent.Name()
		if base != "" && !strings.Contains(strings.ToLower(name), lower) && !isSubsequenceFold(name, base) {
			continue
		}
		if ent.IsDir() && workspace.ShouldIgnore(name, m.cfg.Ignore.Dirs) {
			continue
		}
		full := filepath.Join(dir, name)
		rel := full
		if r, err := filepath.Rel(m.walkRoot(), full); err == nil && !strings.HasPrefix(r, "..") {
			rel = r
		}
		if ent.IsDir() {
			rel += "/"
		}
		info, _ := ent.Info()
		var size int64
		if info != nil {
			size = info.Size()
		}
		out = append(out, workspace.Match{File: workspace.File{
			Path: full, Rel: rel, Size: size, IsDir: ent.IsDir(),
		}})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].File.IsDir != out[j].File.IsDir {
			return out[i].File.IsDir
		}
		return strings.ToLower(out[i].File.Rel) < strings.ToLower(out[j].File.Rel)
	})
	if len(out) > 200 {
		out = out[:200]
	}
	return out
}

func isSubsequenceFold(s, q string) bool {
	return isSubsequence(strings.ToLower(s), strings.ToLower(q))
}

func (m *Model) openKey(msg tea.KeyPressMsg) tea.Cmd {
	k := msg.Keystroke()
	switch k {
	case "esc":
		m.closeOverlays()
		return nil
	case "enter":
		return m.chooseOpen()
	case "up":
		m.openMove(-1)
		return nil
	case "down":
		m.openMove(1)
		return nil
	case "pgup":
		m.openMove(-openListLimit)
		return nil
	case "pgdown":
		m.openMove(openListLimit)
		return nil
	case "ctrl+n":
		return m.createFileFromQuery()
	case "tab":
		m.openComplete()
		return nil
	case "ctrl+u":
		m.ov.input.Clear()
		m.recomputeOpenMatches()
		m.ov.sel = 0
		return nil
	case "ctrl+w":
		m.ov.input.DeleteWordLeft()
		m.recomputeOpenMatches()
		m.ov.sel = 0
		return nil
	case "backspace":
		m.ov.input.Backspace()
		m.recomputeOpenMatches()
		m.ov.sel = 0
		return nil
	case "delete":
		m.ov.input.Delete()
		m.recomputeOpenMatches()
		m.ov.sel = 0
		return nil
	case "left":
		m.ov.input.MoveLeft()
		return nil
	case "right":
		m.ov.input.MoveRight()
		return nil
	case "ctrl+left":
		m.ov.input.WordLeft()
		return nil
	case "ctrl+right":
		m.ov.input.WordRight()
		return nil
	case "home", "ctrl+a":
		m.ov.input.Home()
		return nil
	case "end":
		m.ov.input.End()
		return nil
	}
	if printable(msg) {
		m.ov.input.Insert(msg.Text)
		m.recomputeOpenMatches()
		m.ov.sel = 0
	}
	return nil
}

func (m *Model) openMove(delta int) {
	n := len(m.ov.matches)
	if n == 0 {
		return
	}
	m.ov.sel = (m.ov.sel + delta%n + n) % n
}

func (m *Model) openComplete() {
	if m.ov.sel < 0 || m.ov.sel >= len(m.ov.matches) {
		return
	}
	m.ov.input.Set(m.ov.matches[m.ov.sel].File.Rel)
	m.ov.input.End()
	m.recomputeOpenMatches()
}

func (m *Model) chooseOpen() tea.Cmd {
	if m.ov.sel >= 0 && m.ov.sel < len(m.ov.matches) {
		f := m.ov.matches[m.ov.sel].File
		if f.IsDir {
			m.ov.input.Set(f.Path + "/")
			m.recomputeOpenMatches()
			m.ov.sel = 0
			return nil
		}
		m.closeOverlays()
		m.openFile(f.Path)
		return nil
	}
	q := strings.TrimSpace(m.ov.input.Value())
	if q == "" {
		m.notifyInfo(m.tr("введите путь или выберите файл", "enter a path or choose a file"))
		return nil
	}
	path := m.resolvePath(q)
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		m.ov.input.Set(path + "/")
		m.recomputeOpenMatches()
		return nil
	}
	m.closeOverlays()
	m.openFile(path)
	return nil
}

func (m *Model) resolvePath(path string) string {
	path = expandHome(path)
	if !filepath.IsAbs(path) {
		path = filepath.Join(m.walkRoot(), path)
	}
	return filepath.Clean(path)
}

func (m *Model) createFileFromQuery() tea.Cmd {
	q := strings.TrimSpace(m.ov.input.Value())
	if q == "" {
		m.notifyError(m.tr("введите путь нового файла", "enter a path for the new file"))
		return nil
	}
	path := m.resolvePath(q)
	if info, err := os.Stat(path); err == nil {
		if info.IsDir() {
			m.notifyError(m.tr("это папка", "this is a directory"))
			return nil
		}
		m.closeOverlays()
		m.openFile(path)
		return nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		m.notifyError(m.tr("не создано: ", "not created: ") + err.Error())
		return nil
	}
	f.Close()
	m.openFile(path)
	m.notifySuccess(m.tr("создан ", "created ") + path)
	return nil
}

// openFile открывает файл в пустой текущей вкладке или в новой; дубль
// переключает на существующую вкладку.
func (m *Model) openFile(path string) {
	m.closeOverlays()
	path = m.resolvePath(path)
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		m.notifyError(m.tr("это папка: ", "this is a directory: ") + path)
		return
	}
	for i, t := range m.tabs {
		if t.ed.Buf.Path != "" && filepath.Clean(t.ed.Buf.Path) == path {
			m.rememberRecent(path)
			// Пустая вкладка, из которой открывали файл, не должна оставаться.
			cur := m.tab()
			if i != m.active && cur != nil && cur.ed.Buf.Path == "" && !cur.ed.Buf.Dirty() {
				blank := m.active
				m.tabs = append(m.tabs[:blank], m.tabs[blank+1:]...)
				if blank < i {
					i--
				}
			}
			m.active = max(0, min(i, len(m.tabs)-1))
			m.checkExternalActive()
			m.scheduleGhost()
			return
		}
	}
	var (
		buf *ed.Buffer
		err error
	)
	_, statErr := os.Stat(path)
	if statErr == nil {
		buf, err = ed.Open(path)
		if err != nil {
			m.notifyError(m.tr("не открыто: ", "not opened: ") + err.Error())
			return
		}
	} else if os.IsNotExist(statErr) {
		buf = ed.New(path)
	} else {
		m.notifyError(m.tr("не открыто: ", "not opened: ") + statErr.Error())
		return
	}
	nt := m.newTab(buf)
	cur := m.tab()
	if cur != nil && cur.ed.Buf.Path == "" && !cur.ed.Buf.Dirty() {
		m.tabs[m.active] = nt
	} else {
		m.tabs = append(m.tabs, nt)
		m.active = len(m.tabs) - 1
	}
	m.rememberRecent(path)
	if buf.Path == "" {
		nt.ed.Buf.Path = path
	}
	m.closeOverlays()
	m.clampScroll()
	m.scheduleGhost()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		m.notifyInfo(m.tr("новый файл: ", "new file: ") + path)
	}
}

// ---- отрисовка палитры открытия ---------------------------------------

type openView struct {
	shown     bool
	box       []string
	x, y      int
	itemTop   int
	itemCount int
	inner     int
}

func (m *Model) openVisibleCount() int {
	if m.w < 40 || m.h < 10 {
		return 3
	}
	return min(openListLimit, max(1, m.h-5))
}

func (m *Model) openStart() int {
	limit := m.openVisibleCount()
	if m.ov.sel >= limit {
		return m.ov.sel - limit + 1
	}
	return 0
}

func (m *Model) openView() openView {
	if m.ov.kind != overlayOpen {
		return openView{}
	}
	if m.w < 40 || m.h < 10 {
		// Компактный режим: без рамки, но пользоваться можно.
		inputY := m.h - 2
		n := min(len(m.ov.matches), m.openVisibleCount(), max(0, inputY))
		top := inputY - n
		if top < 0 {
			top = 0
		}
		start := m.openStart()
		rows := make([]string, 0, n+1)
		for j := 0; j < n; j++ {
			rows = append(rows, m.openItemRow(start+j, m.w))
		}
		rows = append(rows, m.inputOverlayRow("> ", &m.ov.input))
		v := openView{shown: true, box: rows, x: 0, y: top, itemTop: top, itemCount: n, inner: m.w}
		return v
	}

	inner := min(m.w-6, 72)
	rows := []string{m.openInputRow(inner), m.th.panelBorder + strings.Repeat("─", inner) + sgrReset}
	n := min(len(m.ov.matches), m.openVisibleCount())
	start := m.openStart()
	for j := 0; j < n; j++ {
		rows = append(rows, m.openItemRow(start+j, inner))
	}
	if n == 0 {
		rows = append(rows, m.th.panelDim+padANSI("  "+m.tr("нет файлов · Ctrl-N создать", "no files · Ctrl-N to create"), inner, m.th.panel)+sgrReset)
	}
	footer := m.openFooter()
	box := widgets.RenderBox(widgets.Box{
		Title: m.tr("Открыть", "Open"), Rows: rows, Footer: footer, Width: m.w,
	}, m.th.widgetStyle())
	x := max(0, (m.w-(inner+2))/2)
	y := min(2, max(0, m.h-len(box)))
	return openView{
		shown: true, box: box, x: x, y: y,
		itemTop:   y + 3, // рамка + ввод + разделитель
		itemCount: n, inner: inner,
	}
}

func (m *Model) openInputRow(inner int) string {
	w := inner
	if w <= 0 {
		w = max(1, m.w-8)
	}
	return m.th.panel + "> " + renderInput(&m.ov.input, max(1, w-2), m.th.panelOn, m.th.panelOn+m.th.cursor) + sgrReset
}

func (m *Model) openItemRow(i, width int) string {
	if i < 0 || i >= len(m.ov.matches) {
		return ""
	}
	mt := m.ov.matches[i]
	style := m.th.panelOn
	mark := "  "
	if i == m.ov.sel {
		style = m.th.panelSel
		mark = "▸ "
	}
	size := formatSize(mt.File.Size)
	if mt.File.IsDir {
		size = m.tr("папка", "dir")
	}
	right := " " + size + " "
	leftW := max(0, width-ansiWidth(mark)-ansiWidth(right))
	left := truncateTo(sanitizeOneLine(mt.File.Rel), leftW)
	line := mark + padANSI(left, leftW, "") + right
	return style + padANSI(line, width, "") + sgrReset
}

func (m *Model) openFooter() string {
	loading := ""
	if m.ov.loading {
		loading = "…"
	}
	shown := len(m.ov.matches)
	if shown > m.openVisibleCount() {
		shown = m.openVisibleCount()
	}
	counter := m.trf("%d из %d%s", "%d of %d%s", shown, m.ov.total, loading)
	if m.w < 60 {
		return counter + " · ↑↓ · ↵ · Esc"
	}
	return counter + m.tr(" · ↑↓ выбрать · ↵ открыть · Ctrl-N создать · Esc закрыть",
		" · ↑↓ select · ↵ open · Ctrl-N create · Esc close")
}

func formatSize(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1fM", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0fK", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%dB", n)
	}
}
