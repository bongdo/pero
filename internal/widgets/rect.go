// Package widgets — UI-примитивы без знания о модели редактора.
//
// Ширина и обрезка считаются в ячейках терминала через ansi.StringWidth/
// ansi.Truncate/ansi.Cut, поэтому эмодзи и кириллица не ломают раскладку.
package widgets

// Rect — полуоткрытый прямоугольник в ячейках терминала: [X, X+W) × [Y, Y+H).
type Rect struct {
	X, Y, W, H int
}

// Contains сообщает, попадает ли ячейка (x, y) внутрь прямоугольника.
func (r Rect) Contains(x, y int) bool {
	return r.W > 0 && r.H > 0 &&
		x >= r.X && x < r.X+r.W &&
		y >= r.Y && y < r.Y+r.H
}

// Empty сообщает, что прямоугольник не занимает ни одной ячейки.
func (r Rect) Empty() bool {
	return r.W <= 0 || r.H <= 0
}

// Intersect возвращает пересечение прямоугольников. Если пересечения нет,
// возвращается нулевой Rect.
func (r Rect) Intersect(o Rect) Rect {
	if r.Empty() || o.Empty() {
		return Rect{}
	}
	x1, y1 := max(r.X, o.X), max(r.Y, o.Y)
	x2, y2 := min(r.X+r.W, o.X+o.W), min(r.Y+r.H, o.Y+o.H)
	if x2 <= x1 || y2 <= y1 {
		return Rect{}
	}
	return Rect{x1, y1, x2 - x1, y2 - y1}
}

// Center ставит прямоугольник boxW×boxH по центру экрана w×h. Если он больше
// экрана, координаты прижимаются к 0, чтобы попап не ушёл в отрицательные
// значения. Вызывающий сам обрезает размеры.
func Center(w, h, boxW, boxH int) (x, y int) {
	if w <= 0 || h <= 0 || boxW <= 0 || boxH <= 0 {
		return 0, 0
	}
	x, y = (w-boxW)/2, (h-boxH)/2
	return max(0, x), max(0, y)
}

// PlaceBelow ставит попап под якорем (anchorX, anchorY). Если снизу не хватает
// места — раскрывает вверх, если справа — сдвигает влево. Результат всегда
// внутри терминала. В терминале меньше 40×10 попап не показывается: (0, 0).
func PlaceBelow(anchorX, anchorY, boxW, boxH, termW, termH int) (x, y int) {
	if termW < 40 || termH < 10 || boxW <= 0 || boxH <= 0 {
		return 0, 0
	}
	if boxW > termW {
		boxW = termW
	}
	if boxH > termH {
		boxH = termH
	}

	x = max(0, anchorX)
	if x+boxW > termW {
		x = termW - boxW
	}

	y = anchorY + 1
	if y+boxH > termH {
		y = anchorY - boxH
	}
	if y > termH-boxH {
		y = termH - boxH
	}
	if y < 0 {
		y = 0
	}
	return x, y
}
