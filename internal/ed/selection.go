package ed

// Selection — якорь и курсор выделения. Диапазон — полуинтервал [min, max).
// Направление запоминается в Anchor/Cursor для Shift-расширения.
type Selection struct {
	Anchor, Cursor Pos
	Active         bool
}
