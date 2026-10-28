package workspace

import (
	"cmp"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

// Match - результат фильтра. Positions - индексы рун в File.Rel; по ним
// рисуется подсветка.
type Match struct {
	File      File
	Score     int
	Positions []int
}

// defaultFuzzyLimit - сколько совпадений отдавать, если limit не задан.
const defaultFuzzyLimit = 200

// fuzzyNameTier держит совпадения по имени выше любых совпадений по пути.
const fuzzyNameTier = 1 << 20

// Filter - регистронезависимый fuzzy-поиск по Rel и имени файла.
// Пустой query возвращает первые limit файлов в исходном порядке.
func Filter(files []File, query string, limit int) []Match {
	if limit <= 0 {
		limit = defaultFuzzyLimit
	}
	matches := make([]Match, 0, min(limit, len(files)))
	folded := foldRunes([]rune(query))
	if len(folded) == 0 {
		for _, file := range files {
			if len(matches) >= limit {
				break
			}
			matches = append(matches, Match{File: file})
		}
		return matches
	}

	for _, file := range files {
		if match, ok := matchFile(file, folded); ok {
			matches = append(matches, match)
		}
	}
	slices.SortFunc(matches, func(a, b Match) int {
		if c := cmp.Compare(b.Score, a.Score); c != 0 {
			return c
		}
		if c := strings.Compare(a.File.Rel, b.File.Rel); c != 0 {
			return c
		}
		return strings.Compare(a.File.Path, b.File.Path)
	})
	if len(matches) > limit {
		matches = matches[:limit]
	}
	return matches
}

// matchFile сначала пробует имя файла, потом весь Rel.
func matchFile(file File, query []rune) (Match, bool) {
	rel := file.Rel
	if rel == "" {
		rel = file.Path
	}
	relRunes := []rune(rel)
	nameRunes := []rune(filepath.Base(rel))

	if m, ok := matchRunes(nameRunes, query); ok {
		offset := max(0, len(relRunes)-len(nameRunes))
		positions := make([]int, len(m.positions))
		for i, pos := range m.positions {
			positions[i] = pos + offset
		}
		return Match{File: file, Score: fuzzyNameTier + m.score, Positions: positions}, true
	}
	if m, ok := matchRunes(relRunes, query); ok {
		return Match{File: file, Score: m.score, Positions: m.positions}, true
	}
	return Match{}, false
}

type runeMatch struct {
	positions []int
	score     int
}

// matchRunes ищет query как подпоследовательность в text (оба уже в нижнем
// регистре) и оценивает качество: префикс, границы слов, слитность, разрывы.
func matchRunes(text, query []rune) (runeMatch, bool) {
	if len(query) == 0 || len(query) > len(text) {
		return runeMatch{}, false
	}
	positions := make([]int, 0, len(query))
	at := 0
	for _, q := range query {
		for at < len(text) && foldRune(text[at]) != q {
			at++
		}
		if at == len(text) {
			return runeMatch{}, false
		}
		positions = append(positions, at)
		at++
	}

	score := 0
	if positions[0] == 0 {
		score += 500
	}
	if len(positions) == len(text) {
		score += 400
	}
	for _, pos := range positions {
		if pos == 0 || isWordBoundary(text[pos-1]) {
			score += 40
		}
	}
	for i := 1; i < len(positions); i++ {
		if positions[i] == positions[i-1]+1 {
			score += 25
		}
	}
	gaps := positions[len(positions)-1] - positions[0] + 1 - len(positions)
	score -= min(gaps*2, 100)
	score -= min(len(text)/4, 100)

	return runeMatch{positions: positions, score: score}, true
}

// foldRune приводит руну к нижнему регистру; ё и е считаются одной буквой.
func foldRune(r rune) rune {
	r = unicode.ToLower(r)
	if r == 'ё' {
		return 'е'
	}
	return r
}

func foldRunes(in []rune) []rune {
	out := make([]rune, len(in))
	for i, r := range in {
		out[i] = foldRune(r)
	}
	return out
}

func isWordBoundary(r rune) bool {
	switch r {
	case '/', '\\', '_', '-', '.':
		return true
	}
	return unicode.IsSpace(r)
}
