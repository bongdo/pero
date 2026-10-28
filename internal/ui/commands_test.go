package ui

import "testing"

func TestFilterCommands(t *testing.T) {
	all := []command{
		{id: "file.save", title: "Сохранить файл", key: "Ctrl-S"},
		{id: "file.open", title: "Открыть файл по пути", key: "Ctrl-P"},
		{id: "app.quit", title: "Выйти из редактора", key: "Ctrl-Q"},
	}

	cases := []struct {
		query string
		want  string
	}{
		{"save", "file.save"},
		{"сохр", "file.save"},
		{"quit", "app.quit"},
		{"откр", "file.open"},
		{"fs", "file.save"}, // подпоследовательность в id
	}
	for _, c := range cases {
		got := filterCommands(all, c.query)
		if len(got) == 0 || got[0].id != c.want {
			t.Fatalf("query %q: got %v, want first %q", c.query, ids(got), c.want)
		}
	}

	if got := filterCommands(all, "zzzz"); len(got) != 0 {
		t.Fatalf("мусорный запрос что-то нашёл: %v", ids(got))
	}
}

func TestFilterCommandsEmptyQuery(t *testing.T) {
	all := []command{{id: "a", title: "A"}, {id: "b", title: "B"}}
	if got := filterCommands(all, "  "); len(got) != 2 {
		t.Fatalf("пустой запрос должен вернуть всё, got %d", len(got))
	}
}

func ids(cs []command) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.id
	}
	return out
}
