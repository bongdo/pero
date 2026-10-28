package widgets

import "testing"

func TestRectContainsAndEmpty(t *testing.T) {
	r := Rect{X: 2, Y: 3, W: 4, H: 2}
	for _, p := range []struct {
		x, y int
		want bool
	}{
		{2, 3, true},
		{5, 4, true},
		{1, 3, false},
		{2, 2, false},
		{6, 3, false},
		{2, 5, false},
	} {
		if got := r.Contains(p.x, p.y); got != p.want {
			t.Errorf("Contains(%d,%d) = %v, want %v", p.x, p.y, got, p.want)
		}
	}
	if (Rect{W: 0, H: 3}).Empty() != true || (Rect{W: 3, H: -1}).Empty() != true {
		t.Error("rect with non-positive size must be empty")
	}
	if r.Empty() {
		t.Error("non-empty rect reported empty")
	}
}

func TestRectIntersect(t *testing.T) {
	a := Rect{X: 0, Y: 0, W: 10, H: 5}
	if got := a.Intersect(Rect{X: 5, Y: 2, W: 10, H: 5}); got != (Rect{X: 5, Y: 2, W: 5, H: 3}) {
		t.Errorf("intersect = %+v", got)
	}
	if got := a.Intersect(Rect{X: 20, Y: 0, W: 5, H: 5}); !got.Empty() {
		t.Errorf("disjoint intersect = %+v, want empty", got)
	}
	if got := a.Intersect(Rect{}); !got.Empty() {
		t.Errorf("empty intersect = %+v, want empty", got)
	}
}

func TestCenter(t *testing.T) {
	if x, y := Center(80, 24, 20, 6); x != 30 || y != 9 {
		t.Errorf("Center = %d,%d, want 30,9", x, y)
	}
	if x, y := Center(80, 24, 100, 100); x != 0 || y != 0 {
		t.Errorf("oversized Center = %d,%d, want 0,0", x, y)
	}
	if x, y := Center(0, 0, 10, 10); x != 0 || y != 0 {
		t.Errorf("zero terminal Center = %d,%d, want 0,0", x, y)
	}
}

func TestPlaceBelow(t *testing.T) {
	const termW, termH = 80, 24
	cases := []struct {
		name           string
		ax, ay, bw, bh int
		wantX, wantY   int
	}{
		{"fits below", 10, 5, 20, 6, 10, 6},
		{"flips up", 10, 20, 20, 6, 10, 14},
		{"shifts left", 70, 5, 20, 6, 60, 6},
		{"bottom right", 70, 20, 20, 6, 60, 14},
		{"negative anchor", -5, -5, 20, 6, 0, 0},
		{"below screen", 10, 100, 20, 6, 10, 18},
	}
	for _, c := range cases {
		x, y := PlaceBelow(c.ax, c.ay, c.bw, c.bh, termW, termH)
		if x != c.wantX || y != c.wantY {
			t.Errorf("%s: got %d,%d, want %d,%d", c.name, x, y, c.wantX, c.wantY)
		}
	}

	if x, y := PlaceBelow(10, 5, 20, 6, 39, 24); x != 0 || y != 0 {
		t.Errorf("narrow terminal = %d,%d, want 0,0", x, y)
	}
	if x, y := PlaceBelow(10, 5, 20, 6, 80, 9); x != 0 || y != 0 {
		t.Errorf("short terminal = %d,%d, want 0,0", x, y)
	}

	// Никогда не выходим за экран (в том числе для попапа больше экрана).
	for _, bw := range []int{10, 30, 100} {
		for _, bh := range []int{2, 8, 40} {
			for ax := -10; ax <= termW+10; ax += 7 {
				for ay := -10; ay <= termH+10; ay += 5 {
					x, y := PlaceBelow(ax, ay, bw, bh, termW, termH)
					if bw <= termW && bh <= termH {
						if x < 0 || y < 0 || x+bw > termW || y+bh > termH {
							t.Fatalf("out of bounds: anchor %d,%d box %dx%d -> %d,%d", ax, ay, bw, bh, x, y)
						}
					}
				}
			}
		}
	}
}
