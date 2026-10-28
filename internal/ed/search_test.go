package ed

import "testing"

func TestFindAllLiteralAndFold(t *testing.T) {
	e := testEditor("f.md", "Привет, мир! привет МИР!")
	if got := e.FindAll("привет", true); len(got) != 2 {
		t.Fatalf("fold: %d совпадений %v", len(got), got)
	}
	if got := e.FindAll("привет", false); len(got) != 1 || got[0].Start != (Pos{0, 13}) {
		t.Fatalf("literal: %v", got)
	}
	if got := e.FindAll("ё", true); len(got) != 0 {
		t.Fatalf("fold не должен приравнивать ё к е: %v", got)
	}
	if got := e.FindAll("", true); got != nil {
		t.Fatalf("пустой запрос = %v", got)
	}
	if got := e.FindAll("zzz", false); len(got) != 0 {
		t.Fatalf("нет совпадений: %v", got)
	}
}

func TestFindAllCyrillicPositions(t *testing.T) {
	e := testEditor("f.txt", "ёж\nЁЖ\nёж")
	got := e.FindAll("ёж", true)
	want := []Range{
		{Start: Pos{0, 0}, End: Pos{0, 2}},
		{Start: Pos{1, 0}, End: Pos{1, 2}},
		{Start: Pos{2, 0}, End: Pos{2, 2}},
	}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%d: %v != %v", i, got[i], want[i])
		}
	}
}

func TestFindNextPrevCircular(t *testing.T) {
	e := testEditor("f.txt", "аа аа аа")
	all := e.FindAll("аа", false)
	if len(all) != 3 {
		t.Fatalf("найдено %d", len(all))
	}
	for i, m := range all {
		from := Pos{0, 0}
		if i > 0 {
			from = all[i-1].End
		}
		got, ok := e.FindNext("аа", from, false)
		if !ok || got != m {
			t.Fatalf("next %d: %v %v", i, got, ok)
		}
	}
	// После последнего — по кругу первый.
	got, ok := e.FindNext("аа", all[2].End, false)
	if !ok || got != all[0] {
		t.Fatalf("wrap next: %v %v", got, ok)
	}
	// Назад от первого — последний.
	got, ok = e.FindPrev("аа", all[0].Start, false)
	if !ok || got != all[2] {
		t.Fatalf("wrap prev: %v %v", got, ok)
	}
	for i := len(all) - 1; i > 0; i-- {
		got, ok := e.FindPrev("аа", all[i].Start, false)
		if !ok || got != all[i-1] {
			t.Fatalf("prev %d: %v %v", i, got, ok)
		}
	}
	if _, ok := e.FindNext("нет", Pos{}, false); ok {
		t.Fatal("FindNext без совпадений = true")
	}
	if _, ok := e.FindPrev("нет", Pos{}, false); ok {
		t.Fatal("FindPrev без совпадений = true")
	}
	if _, ok := e.FindNext("", Pos{}, false); ok {
		t.Fatal("FindNext с пустым запросом = true")
	}
}

func TestFindNonOverlappingAndMultiline(t *testing.T) {
	e := testEditor("f.txt", "aaaa")
	if got := e.FindAll("aa", false); len(got) != 2 {
		t.Fatalf("пересечения: %v", got)
	}

	e = testEditor("f.txt", "один\nдва\nтри")
	got, ok := e.FindNext("ин\nдв", Pos{}, false)
	if !ok || got.Start != (Pos{0, 2}) || got.End != (Pos{1, 2}) {
		t.Fatalf("многострочный: %v %v", got, ok)
	}
	if text := e.Buf.textRange(got); text != "ин\nдв" {
		t.Fatalf("textRange = %q", text)
	}
	// Многострочный поиск с регистром.
	e = testEditor("f.txt", "АБ\nВГ")
	if got := e.FindAll("аб\nвг", true); len(got) != 1 || got[0].Start != (Pos{0, 0}) {
		t.Fatalf("fold multiline: %v", got)
	}
}

func TestFindDoesNotMoveCursor(t *testing.T) {
	e := testEditor("f.txt", "микро микро")
	e.SetCursor(Pos{0, 3})
	_ = e.FindAll("микро", false)
	if _, ok := e.FindNext("микро", e.Cursor, false); !ok {
		t.Fatal("FindNext = false")
	}
	if e.Cursor != (Pos{0, 3}) || e.HasSelection() {
		t.Fatalf("поиск сдвинул курсор: %v %+v", e.Cursor, e.Sel)
	}
}

func TestFindMatchesAtNewlineBoundary(t *testing.T) {
	cases := []struct {
		text, query string
		want        []Range
	}{
		{
			text:  "a\nb\n",
			query: "\n",
			want: []Range{
				{Start: Pos{0, 1}, End: Pos{1, 0}},
				{Start: Pos{1, 1}, End: Pos{2, 0}},
			},
		},
		{
			text:  "a\nb",
			query: "\nb",
			want:  []Range{{Start: Pos{0, 1}, End: Pos{1, 1}}},
		},
		{
			text:  "\n\n",
			query: "\n\n",
			want:  []Range{{Start: Pos{0, 0}, End: Pos{2, 0}}},
		},
	}
	for _, c := range cases {
		e := testEditor("f.txt", c.text)
		got := e.FindAll(c.query, false)
		if len(got) != len(c.want) {
			t.Fatalf("%q in %q: got %v, want %v", c.query, c.text, got, c.want)
		}
		for i := range c.want {
			if got[i] != c.want[i] {
				t.Fatalf("%q in %q: match %d = %v, want %v", c.query, c.text, i, got[i], c.want[i])
			}
			if text := e.Buf.textRange(got[i]); text != c.query {
				t.Fatalf("%q: textRange = %q", c.query, text)
			}
		}
	}
}
