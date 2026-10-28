package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bongdo/pero/internal/config"
	"github.com/bongdo/pero/internal/widgets"
)

// sgrTheme - готовые ANSI-префиксы для горячего рендера. Пустая строка значит
// «без стиля». lipgloss в этом файле не используется намеренно.
type sgrTheme struct {
	bg, panel, border, text, muted, accent string
	sel, curLine, gutter                   string
	statusBG, statusFG                     string
	tabActive, tabInactive                 string
	error, warning, info, ghost            string
	searchBG                               string
	diagError, diagWarning, diagInfo       string

	base, current, selected, status, dim     string
	accentOn, errOn, warnOn, infoOn, ghostOn string
	search, searchCur, cursor                string
	bracketOn                                string
	tabBar, tabActiveOn, tabInactiveOn       string
	panelOn, panelDim, panelBorder, panelSel string
	panelErr, panelWarn                      string
	gutterOn, baseOn, curOn, selectionOn     string
}

func newSGRTheme(c config.Theme) sgrTheme {
	th := sgrTheme{
		bg: bgSGR(c.BG), panel: bgSGR(c.Panel), border: fgSGR(c.Border),
		text: fgSGR(c.Text), muted: fgSGR(c.Muted), accent: fgSGR(c.Accent),
		sel: bgSGR(c.SelectionBG), curLine: bgSGR(c.CurrentLine), gutter: fgSGR(c.Gutter),
		statusBG: bgSGR(c.StatusBG), statusFG: fgSGR(c.StatusFG),
		tabActive: fgSGR(c.TabActive), tabInactive: fgSGR(c.TabInactive),
		error: fgSGR(c.Error), warning: fgSGR(c.Warning), info: fgSGR(c.Info),
		ghost:     fgSGR(c.Ghost),
		searchBG:  bgSGR(c.SearchBG),
		diagError: fgSGR(c.DiagnosticError), diagWarning: fgSGR(c.DiagnosticWarning),
		diagInfo: fgSGR(c.DiagnosticInfo),
	}
	// Фон текста - отдельным префиксом: если тема его не задаёт, остаётся
	// терминальный фон.
	baseBG := bgSGR(c.BG)
	th.base = baseBG + th.text
	th.baseOn = th.base
	th.current = th.curLine + th.text
	th.curOn = th.current
	th.selected = th.sel + th.text
	th.selectionOn = th.selected
	th.status = th.statusBG + th.statusFG
	th.dim = baseBG + th.muted
	th.accentOn = baseBG + th.accent
	th.errOn = baseBG + th.error
	th.warnOn = baseBG + th.warning
	th.infoOn = baseBG + th.info
	th.ghostOn = baseBG + th.ghost
	th.gutterOn = baseBG + th.gutter
	th.search = th.searchBG + th.text
	th.searchCur = bgSGR(c.SearchCurrent) + fgSGR(c.BG) + "\x1b[1m"
	th.bracketOn = th.sel + th.accent + "\x1b[1m"
	th.cursor = "\x1b[7m"
	th.tabBar = bgSGR(c.BG)
	th.tabActiveOn = th.tabBar + th.tabActive + "\x1b[1m"
	th.tabInactiveOn = th.tabBar + th.tabInactive
	th.panelOn = th.panel + th.text
	th.panelDim = th.panel + th.muted
	th.panelBorder = th.panel + th.border
	th.panelSel = th.sel + th.text
	th.panelErr = th.panel + th.error
	th.panelWarn = th.panel + th.warning
	return th
}

func (th sgrTheme) widgetStyle() widgets.Style {
	return widgets.Style{
		BG:         th.panel,
		Border:     th.panelBorder,
		Text:       th.panelOn,
		Muted:      th.panelDim,
		SelectedBG: th.sel,
		SelectedFG: th.panelOn,
		Error:      th.panelErr,
		Warning:    th.panelWarn,
	}
}

// diagUnderline - SGR подчёркивания, не меняющий цвет текста: 4 + цвет линии.
func (th sgrTheme) diagUnderline(severity int, current bool) string {
	c := th.diagInfo
	switch severity {
	case 1:
		c = th.diagError
	case 2:
		c = th.diagWarning
	}
	if c == "" {
		return ""
	}
	u := underlineColorSGR(strings.TrimPrefix(c, "\x1b["))
	if u == "" {
		return "\x1b[4m"
	}
	if current {
		return "\x1b[4m" + u + "\x1b[1m"
	}
	return "\x1b[4m" + u
}

func underlineColorSGR(raw string) string {
	// raw уже без ESC[: "38;2;r;g;bm" или "38;5;nm" или "3Xm".
	raw = strings.TrimSuffix(raw, "m")
	parts := strings.Split(raw, ";")
	if len(parts) >= 2 {
		if parts[0] == "38" {
			switch parts[1] {
			case "2":
				if len(parts) >= 5 {
					return "\x1b[58;2;" + strings.Join(parts[2:5], ";") + "m"
				}
			case "5":
				if len(parts) >= 3 {
					return "\x1b[58;5;" + parts[2] + "m"
				}
			}
		}
	}
	if len(parts) == 1 {
		if n, err := strconv.Atoi(parts[0]); err == nil {
			switch {
			case n >= 30 && n <= 37:
				return fmt.Sprintf("\x1b[58;5;%dm", n-30)
			case n >= 90 && n <= 97:
				return fmt.Sprintf("\x1b[58;5;%dm", n-90+8)
			}
		}
	}
	return ""
}

// fgSGR превращает hex или номер ANSI в SGR цвета текста.
func fgSGR(v string) string {
	return colorSGR(v, false)
}

func bgSGR(v string) string {
	return colorSGR(v, true)
}

func colorSGR(v string, bg bool) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if strings.HasPrefix(v, "#") && len(v) == 7 {
		r, err1 := strconv.ParseInt(v[1:3], 16, 0)
		g, err2 := strconv.ParseInt(v[3:5], 16, 0)
		b, err3 := strconv.ParseInt(v[5:7], 16, 0)
		if err1 == nil && err2 == nil && err3 == nil {
			if bg {
				return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r, g, b)
			}
			return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", r, g, b)
		}
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return ""
	}
	switch {
	case n < 8:
		if bg {
			return fmt.Sprintf("\x1b[%dm", 40+n)
		}
		return fmt.Sprintf("\x1b[%dm", 30+n)
	case n < 16:
		if bg {
			return fmt.Sprintf("\x1b[%dm", 100+n-8)
		}
		return fmt.Sprintf("\x1b[%dm", 90+n-8)
	default:
		if bg {
			return fmt.Sprintf("\x1b[48;5;%dm", n)
		}
		return fmt.Sprintf("\x1b[38;5;%dm", n)
	}
}
