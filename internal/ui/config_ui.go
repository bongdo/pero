package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/bongdo/pero/internal/config"
)

// reloadConfig перечитывает config.toml и применяет настройки к открытым
// вкладкам. Тему не сохраняет - только читает.
func (m *Model) reloadConfig() tea.Cmd {
	cfg, err := config.Load()
	if err != nil {
		m.notifyError(m.tr("конфиг: ", "config: ") + err.Error())
		return nil
	}
	cfg.Normalize()
	m.cfg = cfg
	m.applyTheme(cfg.Theme)
	for _, t := range m.tabs {
		t.ed.TabWidth = cfg.Editor.Tabsize
		t.wrap = cfg.SoftwrapFor(t.ed.Buf.Path)
		t.wrapCache.valid = false
	}
	if !cfg.Editor.Ghost {
		m.cancelGhost()
	} else {
		m.scheduleGhost()
	}
	warns := m.buildBindings()
	if len(warns) > 0 {
		m.notifyWarn(strings.Join(warns, "; "))
		return nil
	}
	m.notifySuccess(m.tr("конфиг перечитан", "config reloaded"))
	return nil
}
