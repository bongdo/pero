package assist

import "strings"

// Источники ghost-подсказки. Показывается только один.
const (
	SourceT9  = "t9"
	SourceLSP = "lsp"
)

// Suggestion - ghost-кандидат. Text - кандидат целиком; UI отрезает уже
// набранный префикс, чтобы нарисовать серый суффикс, и вставляет только его.
type Suggestion struct {
	Text    string
	Source  string
	Version int
}

// GhostCandidate выбирает первый подходящий кандидат из слов буфера (t9).
func GhostCandidate(prefix string, candidates []string, minLen int) (Suggestion, bool) {
	return GhostCandidateFrom(SourceT9, prefix, candidates, minLen)
}

// GhostCandidateFrom - то же с явным источником: SourceT9 или SourceLSP.
// minLen - минимальная длина кандидата в рунах, но не меньше 2.
func GhostCandidateFrom(source, prefix string, candidates []string, minLen int) (Suggestion, bool) {
	if source != SourceT9 && source != SourceLSP {
		return Suggestion{}, false
	}
	if prefix == "" {
		return Suggestion{}, false
	}
	if minLen < 2 {
		minLen = 2
	}

	prefixLen := len([]rune(prefix))
	lower := strings.ToLower(prefix)
	for _, cand := range candidates {
		candLen := len([]rune(cand))
		if candLen < minLen || candLen <= prefixLen {
			continue
		}
		if strings.EqualFold(cand, prefix) {
			continue
		}
		if !strings.HasPrefix(strings.ToLower(cand), lower) {
			continue
		}
		return Suggestion{Text: cand, Source: source}, true
	}
	return Suggestion{}, false
}
