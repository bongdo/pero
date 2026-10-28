package assist

import "testing"

func TestGhostCandidateFilters(t *testing.T) {
	cases := []struct {
		name       string
		prefix     string
		candidates []string
		minLen     int
		want       string
		ok         bool
	}{
		{"пустой префикс", "", []string{"привет"}, 2, "", false},
		{"короткий кандидат", "п", []string{"пр"}, 3, "", false},
		{"кандидат не длиннее", "привет", []string{"при"}, 2, "", false},
		{"равен без учёта регистра", "привет", []string{"ПРИВЕТ"}, 2, "", false},
		{"длиннее префикса", "при", []string{"привет"}, 2, "привет", true},
		{"регистр префикса", "ПРИ", []string{"привет"}, 2, "привет", true},
		{"минимум два символа", "a", []string{"ab"}, 0, "ab", true},
		{"не тот префикс", "при", []string{"мир"}, 2, "", false},
		{"пустой кандидат", "a", []string{"", "ab"}, 2, "ab", true},
		{"первый подходящий", "п", []string{"по", "привет"}, 2, "по", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := GhostCandidate(tc.prefix, tc.candidates, tc.minLen)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if got.Text != tc.want {
				t.Fatalf("Text = %q, want %q", got.Text, tc.want)
			}
			if ok && got.Source != SourceT9 {
				t.Fatalf("Source = %q, want %q", got.Source, SourceT9)
			}
		})
	}
}

func TestGhostCandidateSource(t *testing.T) {
	got, ok := GhostCandidateFrom(SourceLSP, "при", []string{"привет"}, 2)
	if !ok || got.Source != SourceLSP || got.Text != "привет" {
		t.Fatalf("GhostCandidateFrom = %+v, ok=%v", got, ok)
	}
	if _, ok := GhostCandidateFrom("bogus", "при", []string{"привет"}, 2); ok {
		t.Fatal("неизвестный источник должен отбрасываться")
	}
}
