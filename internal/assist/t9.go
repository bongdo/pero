// Package assist — подсказки редактора: t9 по словам буфера, ghost-кандидаты
// и запуск opencode. Пакет не знает про UI, буфер и LSP: только текст на входе.
package assist

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// WordIndex — частоты слов буфера. Хранит только слова и счётчики, не текст.
type WordIndex struct {
	freq map[string]int
}

// NewWordIndex создаёт пустой индекс.
func NewWordIndex() *WordIndex {
	return &WordIndex{freq: make(map[string]int)}
}

// Index пересобирает индекс. Вызывать при смене версии буфера.
func (w *WordIndex) Index(text string) {
	freq := make(map[string]int)

	// Один проход по тексту без []rune: слова — подстроки исходного текста.
	// Ключ копируется только при первом вхождении, чтобы индекс не держал
	// в памяти весь буфер.
	start := -1
	runes := 0
	content := false
	var kinds [256]struct {
		r rune
		k uint8 // 1 — разделитель, 2 — буква, 3 — цифра
	}
	flush := func(end int) {
		if start < 0 {
			return
		}
		addWord(freq, text[start:end], runes, content)
		start, runes, content = -1, 0, false
	}
	for i, r := range text {
		wordRune := true
		switch {
		case r < utf8.RuneSelf:
			switch {
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
				content = true
			case r >= '0' && r <= '9':
			case r == '_' || r == '-':
				content = content || r == '_'
			default:
				wordRune = false
			}
		default:
			slot := &kinds[r&0xFF]
			if slot.r != r {
				slot.r = r
				switch {
				case unicode.IsLetter(r):
					slot.k = 2
				case unicode.IsDigit(r):
					slot.k = 3
				default:
					slot.k = 1
				}
			}
			switch slot.k {
			case 2:
				content = true
			case 3:
			default:
				wordRune = false
			}
		}
		if !wordRune {
			flush(i)
			continue
		}
		if start < 0 {
			start = i
		}
		runes++
	}
	flush(len(text))

	w.freq = freq
}

func addWord(freq map[string]int, raw string, runes int, content bool) {
	word := raw
	// дефис — только внутри слова, не по краям
	if raw[0] == '-' || raw[len(raw)-1] == '-' {
		word = strings.Trim(raw, "-")
		runes = utf8.RuneCountInString(word)
	}
	if runes < 3 || !content {
		return
	}
	if _, ok := freq[word]; ok {
		freq[word]++
		return
	}
	freq[strings.Clone(word)] = 1
}

// Candidates возвращает слова с регистронезависимым префиксом: сначала по
// частоте, затем по алфавиту. limit <= 0 — без ограничения.
func (w *WordIndex) Candidates(prefix string, limit int) []string {
	if w == nil || len(w.freq) == 0 {
		return nil
	}

	lower := strings.ToLower(prefix)
	out := make([]string, 0, len(w.freq))
	for word := range w.freq {
		if strings.HasPrefix(strings.ToLower(word), lower) {
			out = append(out, word)
		}
	}

	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if fa, fb := w.freq[a], w.freq[b]; fa != fb {
			return fa > fb
		}
		la, lb := strings.ToLower(a), strings.ToLower(b)
		if la != lb {
			return la < lb
		}
		return a < b
	})

	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// IsWordRune — символ слова для t9, WordPrefix и выделения.
func IsWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-'
}

// WordPrefix возвращает слово прямо перед курсором. col — индекс руны.
// Дефис внутри слова сохраняется, по краям — отбрасывается.
func WordPrefix(line []rune, col int) string {
	if col < 0 {
		return ""
	}
	if col > len(line) {
		col = len(line)
	}

	start := col
	for start > 0 && IsWordRune(line[start-1]) {
		start--
	}
	for start < col && line[start] == '-' {
		start++
	}

	end := col
	for end > start && line[end-1] == '-' {
		end--
	}
	return string(line[start:end])
}

// TruncateRunes обрезает строку до n рун, не ломая UTF-8.
func TruncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
