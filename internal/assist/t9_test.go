package assist

import (
	"reflect"
	"testing"
)

func TestWordIndexFrequencyAndAlphabet(t *testing.T) {
	w := NewWordIndex()
	w.Index("кот кот кот пёс пёс альфа бета")

	got := w.Candidates("", 10)
	want := []string{"кот", "пёс", "альфа", "бета"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Candidates = %v, want %v", got, want)
	}
}

func TestWordIndexCyrillicAndYo(t *testing.T) {
	w := NewWordIndex()
	w.Index("Ёжик ёжик ёжик привет Привет")

	if got, want := w.Candidates("ёж", 10), []string{"ёжик", "Ёжик"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Candidates(ёж) = %v, want %v", got, want)
	}
	if got, want := w.Candidates("ЁЖ", 10), []string{"ёжик", "Ёжик"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Candidates(ЁЖ) = %v, want %v", got, want)
	}
	if got, want := w.Candidates("при", 10), []string{"Привет", "привет"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Candidates(при) = %v, want %v", got, want)
	}
}

func TestWordIndexFiltersAndHyphen(t *testing.T) {
	w := NewWordIndex()
	w.Index("ab abc 42 123 123abc a-b foo-bar foo_bar -дефис- привет- 42-42")

	got := w.Candidates("", 50)
	set := make(map[string]bool, len(got))
	for _, word := range got {
		set[word] = true
	}
	for _, want := range []string{"abc", "123abc", "a-b", "foo-bar", "foo_bar", "дефис", "привет"} {
		if !set[want] {
			t.Errorf("нет слова %q в %v", want, got)
		}
	}
	for _, bad := range []string{"ab", "42", "123", "42-42", "-дефис-", "привет-"} {
		if set[bad] {
			t.Errorf("лишнее слово %q в %v", bad, got)
		}
	}
}

func TestWordIndexLimitAndReset(t *testing.T) {
	w := NewWordIndex()
	w.Index("альфа альфа бета бета гамма гамма")

	if got, want := w.Candidates("", 2), []string{"альфа", "бета"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("limit 2 = %v, want %v", got, want)
	}
	if got := w.Candidates("", 0); len(got) != 3 {
		t.Fatalf("limit 0 = %v, want 3 слова", got)
	}
	if got := w.Candidates("дел", 5); len(got) != 0 {
		t.Fatalf("нет совпадений, got %v", got)
	}

	w.Index("новое слово")
	if got := w.Candidates("аль", 5); len(got) != 0 {
		t.Fatalf("старый индекс не сброшен: %v", got)
	}
}

func TestWordIndexRuneClassCacheCollision(t *testing.T) {
	w := NewWordIndex()
	// U+2030 и «а» (U+0430) делят младший байт: кэш классов не должен путать
	// пунктуацию с буквой.
	w.Index("альфа\u2030альфа")
	if got, want := w.Candidates("аль", 5), []string{"альфа"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Candidates = %v, want %v", got, want)
	}
}

func TestIsWordRune(t *testing.T) {
	yes := []rune{'a', 'Я', 'ё', 'é', '1', '_', '-'}
	for _, r := range yes {
		if !IsWordRune(r) {
			t.Errorf("IsWordRune(%q) = false, want true", r)
		}
	}
	no := []rune{' ', '\t', '\n', '.', ',', '!', '😀'}
	for _, r := range no {
		if IsWordRune(r) {
			t.Errorf("IsWordRune(%q) = true, want false", r)
		}
	}
}

func TestWordPrefix(t *testing.T) {
	cases := []struct {
		name string
		line string
		col  int
		want string
	}{
		{"начало", "привет мир", 0, ""},
		{"середина", "привет мир", 3, "при"},
		{"конец слова", "привет мир", 6, "привет"},
		{"пробел", "привет мир", 7, ""},
		{"конец строки", "привет мир", 10, "мир"},
		{"за концом", "привет мир", 100, "мир"},
		{"отрицательный", "привет мир", -1, ""},
		{"дефис внутри", "hello-world", 11, "hello-world"},
		{"дефис в конце", "hello-world", 6, "hello"},
		{"дефис в начале", "-слово", 6, "слово"},
		{"только дефисы", "--", 2, ""},
		{"пустая строка", "", 0, ""},
		{"подчёркивание", "привет_", 7, "привет_"},
		{"эмодзи граница", "abc😀def", 3, "abc"},
		{"после эмодзи", "abc😀def", 4, ""},
		{"после эмодзи и буквы", "abc😀def", 5, "d"},
		{"конец после эмодзи", "abc😀def", 7, "def"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := WordPrefix([]rune(tc.line), tc.col); got != tc.want {
				t.Fatalf("WordPrefix(%q, %d) = %q, want %q", tc.line, tc.col, got, tc.want)
			}
		})
	}
}

func TestTruncateRunes(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"привет", 3, "при"},
		{"привет", 0, ""},
		{"привет", -2, ""},
		{"привет", 10, "привет"},
		{"", 5, ""},
		{"a😀b", 2, "a😀"},
		{"a😀b", 1, "a"},
		{"😀😀", 1, "😀"},
	}
	for _, tc := range cases {
		if got := TruncateRunes(tc.in, tc.n); got != tc.want {
			t.Errorf("TruncateRunes(%q, %d) = %q, want %q", tc.in, tc.n, got, tc.want)
		}
	}
}
