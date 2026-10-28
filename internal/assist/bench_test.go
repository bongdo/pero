package assist

import (
	"fmt"
	"strings"
	"testing"
)

func benchmarkText(lines int) string {
	var sb strings.Builder
	for i := 0; i < lines; i++ {
		fmt.Fprintf(&sb, "строка %d содержит несколько разных слов и ещё одно слово\n", i)
	}
	return sb.String()
}

func BenchmarkWordIndex100k(b *testing.B) {
	text := benchmarkText(100_000)
	w := NewWordIndex()
	b.SetBytes(int64(len(text)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Index(text)
	}
}

func BenchmarkWordIndexCandidates(b *testing.B) {
	w := NewWordIndex()
	w.Index(benchmarkText(100_000))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if got := w.Candidates("стр", 20); len(got) == 0 {
			b.Fatal("no candidates")
		}
	}
}
