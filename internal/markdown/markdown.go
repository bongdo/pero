// Package markdown — чистые операции над строками markdown.
// Пакет не знает о буфере и UI: на вход строка, на выход новая строка.
package markdown

import (
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

var proseExt = map[string]bool{
	".md":       true,
	".markdown": true,
	".txt":      true,
	".rst":      true,
	".org":      true,
}

// IsProse — файл считается прозой по расширению (регистр не важен, путь любой).
func IsProse(path string) bool {
	return proseExt[strings.ToLower(filepath.Ext(path))]
}

// IsCheckbox — строка уже чекбокс: после отступа идёт "- [ ] ", "- [x] ",
// "* [ ] ", "+ [x] " или голый "[ ] " / "[x] ".
// Скобка без пробела после "] " чекбоксом не считается.
func IsCheckbox(line string) bool {
	_, _, _, ok := parseCheckbox(line)
	return ok
}

// ToggleCheckbox переключает "- [ ] " ↔ "- [x] " и голый "[ ] " ↔ "[x] ",
// сохраняя отступ и маркер списка. Если чекбокса нет, после ведущего отступа
// вставляется "- [ ] ", остальной текст остаётся на месте.
// Голый чекбокс остаётся голым: дефис не добавляется.
// Функция не знает о code fence; безопасная версия — ToggleCheckboxAt.
func ToggleCheckbox(line string) (string, bool) {
	at, _, checked, ok := parseCheckbox(line)
	if !ok {
		i := leadingSpace(line)
		return line[:i] + "- [ ] " + line[i:], true
	}
	box := "[ ] "
	if !checked {
		box = "[x] "
	}
	return line[:at] + box + line[at+4:], true
}

// ToggleCheckboxAt — ToggleCheckbox с учётом code fence: строки внутри ``` / ~~~
// и сами ограждения не меняются, возвращается ("", false).
func ToggleCheckboxAt(lines [][]rune, line int) (string, bool) {
	if line < 0 || line >= len(lines) {
		return "", false
	}
	if _, _, isFence := fenceRun(lines[line]); isFence {
		return "", false
	}
	if FenceState(lines)[line] {
		return "", false
	}
	return ToggleCheckbox(string(lines[line]))
}

// ListPrefix возвращает маркер списка с пробелом ("- ", "* ", "+ ", "1. ", "12) ")
// без ведущего отступа. ok=false, если строка не начинается с пункта списка.
func ListPrefix(line string) (string, bool) {
	_, marker, _, ok := parseList(line)
	return marker, ok
}

// ContinueList решает, что вставить по Enter. Для непустого пункта списка
// возвращает "\n" + отступ + тот же маркер (номер +1); для пустого пункта —
// clear=true (маркер надо убрать, вставки нет). Для не-списка — ("", false).
func ContinueList(line string) (insert string, clear bool) {
	indent, marker, number, ok := parseList(line)
	if !ok {
		return "", false
	}
	if strings.TrimSpace(line[len(indent)+len(marker):]) == "" {
		return "", true
	}
	if number != "" {
		n, err := strconv.Atoi(number)
		if err == nil {
			marker = strconv.Itoa(n+1) + marker[len(number):]
		}
	}
	return "\n" + indent + marker, false
}

// CollapseSpaces схлопывает пробелы и табы внутри строки в один пробел и
// убирает хвостовые пробелы. Ведущий отступ (пробелы/табы) не меняется;
// строка из одного отступа остаётся как есть.
func CollapseSpaces(line string) string {
	i := leadingSpace(line)
	var b strings.Builder
	b.Grow(len(line))
	b.WriteString(line[:i])
	space := false
	started := false
	for _, r := range line[i:] {
		if r == ' ' || r == '\t' {
			space = true
			continue
		}
		if space && started {
			b.WriteByte(' ')
		}
		space = false
		started = true
		b.WriteRune(r)
	}
	return b.String()
}

// FenceState помечает строки содержимого code fence: true — строка между
// ограждениями, false — снаружи и для самих ограждений (``` и ~~~).
// Незакрытый fence тянется до конца; закрывающим считается ограждение того же
// символа не короче открывающего.
func FenceState(lines [][]rune) []bool {
	state := make([]bool, len(lines))
	var open rune
	openLen := 0
	for i, line := range lines {
		ch, n, isFence := fenceRun(line)
		if open == 0 {
			if isFence {
				open, openLen = ch, n
			}
			continue
		}
		if isFence && ch == open && n >= openLen {
			open, openLen = 0, 0
			continue
		}
		state[i] = true
	}
	return state
}

// Sentence — грубая эвристика "это проза": больше четырёх слов и есть буквы.
func Sentence(text string) bool {
	if len(strings.Fields(text)) <= 4 {
		return false
	}
	for _, r := range text {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

// CyrillicShare — доля кириллических букв среди всех букв (0..1).
// 0, если букв нет. ё/Ё входят в unicode.Cyrillic.
func CyrillicShare(sample string) float64 {
	var letters, cyrillic int
	for _, r := range sample {
		if !unicode.IsLetter(r) {
			continue
		}
		letters++
		if unicode.Is(unicode.Cyrillic, r) {
			cyrillic++
		}
	}
	if letters == 0 {
		return 0
	}
	return float64(cyrillic) / float64(letters)
}

// parseCheckbox возвращает байтовый индекс '[', маркер списка ("- ", "* ", "+ ")
// и состояние. Голый чекбокс даёт пустой маркер.
func parseCheckbox(line string) (at int, marker string, checked, ok bool) {
	i := leadingSpace(line)
	rest := line[i:]
	for _, m := range []string{"- ", "* ", "+ "} {
		if strings.HasPrefix(rest, m) {
			marker = m
			break
		}
	}
	at = i + len(marker)
	rest = line[at:]
	if len(rest) < 4 || rest[0] != '[' || rest[2] != ']' || rest[3] != ' ' {
		return 0, "", false, false
	}
	switch rest[1] {
	case ' ':
		return at, marker, false, true
	case 'x', 'X':
		return at, marker, true, true
	}
	return 0, "", false, false
}

// parseList разбирает пункт списка: отступ, маркер с пробелом и номер
// (пустой для маркированного списка).
func parseList(line string) (indent, marker, number string, ok bool) {
	i := leadingSpace(line)
	indent = line[:i]
	rest := line[i:]
	if len(rest) >= 2 && rest[1] == ' ' && (rest[0] == '-' || rest[0] == '*' || rest[0] == '+') {
		return indent, rest[:2], "", true
	}
	j := 0
	for j < len(rest) && rest[j] >= '0' && rest[j] <= '9' {
		j++
	}
	if j > 0 && j+1 < len(rest) && (rest[j] == '.' || rest[j] == ')') && rest[j+1] == ' ' {
		return indent, rest[:j+2], rest[:j], true
	}
	return indent, "", "", false
}

func leadingSpace(line string) int {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	return i
}

// fenceRun — строка начинает ограждение кода: >=3 подряд ` или ~ после отступа.
func fenceRun(line []rune) (ch rune, n int, ok bool) {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	if i >= len(line) || (line[i] != '`' && line[i] != '~') {
		return 0, 0, false
	}
	ch = line[i]
	for i < len(line) && line[i] == ch {
		i++
		n++
	}
	return ch, n, n >= 3
}
