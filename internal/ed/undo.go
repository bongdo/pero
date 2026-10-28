package ed

import "time"

// Группировка правок: правки в пределах окна объединяются в один шаг undo.
const defaultUndoGroupWindow = 300 * time.Millisecond

// change — одна непрерывная замена: old на new в позиции start.
// Обратная операция — замена new на old.
type change struct {
	start Pos
	old   string
	new   string
}

type editState struct {
	Cursor Pos
	Sel    Selection
}

type undoEntry struct {
	changes   []change
	before    editState
	after     editState
	beforeRev uint64
	afterRev  uint64
	saveGen   int
	at        time.Time
	coalesce  bool
	sealed    bool
}

func (e *Editor) clock() time.Time {
	if e.now != nil {
		return e.now()
	}
	return time.Now()
}

func (e *Editor) groupWindowValue() time.Duration {
	if e.groupWindow > 0 {
		return e.groupWindow
	}
	return defaultUndoGroupWindow
}

func (e *Editor) state() editState {
	sel := e.Sel
	sel.Cursor = e.Cursor
	if !sel.Active {
		sel = Selection{}
	}
	return editState{Cursor: e.Cursor, Sel: sel}
}

func (e *Editor) restore(s editState) {
	e.Cursor = e.Buf.Clamp(s.Cursor)
	if s.Sel.Active {
		anchor := e.Buf.Clamp(s.Sel.Anchor)
		if anchor != e.Cursor {
			e.Sel = Selection{Anchor: anchor, Cursor: e.Cursor, Active: true}
		} else {
			e.Sel = Selection{}
		}
	} else {
		e.Sel = Selection{}
	}
	e.dropGoal()
}

// commit фиксирует уже применённую правку.
func (e *Editor) commit(before editState, beforeRev, afterRev uint64, ch change, coalesce bool) {
	e.redo = nil
	gen := e.Buf.saveGen

	if e.groupDepth > 0 {
		if e.pending == nil {
			e.pending = &undoEntry{before: before, beforeRev: beforeRev, saveGen: gen}
		}
		e.pending.changes = append(e.pending.changes, ch)
		e.pending.after = e.state()
		e.pending.afterRev = afterRev
		e.pending.at = e.clock()
		return
	}

	now := e.clock()
	if coalesce && len(e.undo) > 0 {
		last := &e.undo[len(e.undo)-1]
		if last.coalesce && !last.sealed && last.saveGen == gen &&
			now.Sub(last.at) <= e.groupWindowValue() && e.coalesceChanges(last, ch) {
			last.changes = append(last.changes, ch)
			last.after = e.state()
			last.afterRev = afterRev
			last.at = now
			return
		}
	}
	e.undo = append(e.undo, undoEntry{
		changes:   []change{ch},
		before:    before,
		after:     e.state(),
		beforeRev: beforeRev,
		afterRev:  afterRev,
		saveGen:   gen,
		at:        now,
		coalesce:  coalesce,
	})
}

// coalesceChanges склеивает только соседние однотипные правки: набор в конце
// прошлой вставки, backspace у начала прошлого удаления или повтор Delete.
// Иначе Ctrl-Z после прыжка курсора откатил бы чужие правки.
func (e *Editor) coalesceChanges(last *undoEntry, ch change) bool {
	if len(last.changes) == 0 {
		return false
	}
	prev := last.changes[len(last.changes)-1]
	switch {
	case prev.old == "" && ch.old == "":
		return ch.start == e.Buf.posAfter(prev.start, prev.new)
	case prev.old != "" && ch.old == "":
		// Замена выделения первым символом + продолжение набора.
		return ch.start == e.Buf.posAfter(prev.start, prev.new)
	case prev.new == "" && ch.new == "":
		return ch.start == prev.start || e.Buf.posAfter(ch.start, ch.old) == prev.start
	}
	return false
}

func (e *Editor) sealLast() {
	if len(e.undo) > 0 {
		e.undo[len(e.undo)-1].sealed = true
	}
}

// BeginGroup начинает явную транзакцию. Вложенные вызовы учитываются.
func (e *Editor) BeginGroup() {
	if e.groupDepth == 0 {
		e.sealLast()
		e.pending = &undoEntry{before: e.state(), beforeRev: e.Buf.revision, saveGen: e.Buf.saveGen, at: e.clock()}
	}
	e.groupDepth++
}

// EndGroup закрывает транзакцию. Правки попадают в undo только на внешнем уровне.
func (e *Editor) EndGroup() {
	if e.groupDepth == 0 {
		return
	}
	e.groupDepth--
	if e.groupDepth > 0 {
		return
	}
	p := e.pending
	e.pending = nil
	if p == nil || len(p.changes) == 0 {
		return
	}
	p.after = e.state()
	p.afterRev = e.Buf.revision
	p.coalesce = false
	e.undo = append(e.undo, *p)
}

// Undo откатывает последнюю транзакцию.
func (e *Editor) Undo() bool {
	if e.Buf.ReadOnly || e.groupDepth > 0 || len(e.undo) == 0 {
		return false
	}
	entry := e.undo[len(e.undo)-1]
	e.undo = e.undo[:len(e.undo)-1]
	for i := len(entry.changes) - 1; i >= 0; i-- {
		ch := entry.changes[i]
		r := Range{Start: ch.start, End: e.Buf.posAfter(ch.start, ch.new)}
		e.Buf.replace(r, ch.old)
	}
	e.restore(entry.before)
	e.redo = append(e.redo, entry)
	e.sealLast()
	// Undo возвращает ровно то состояние, что было до транзакции.
	e.Buf.revision = entry.beforeRev
	e.Buf.dirty = e.Buf.revision != e.Buf.savedRevision
	return true
}

// Redo повторяет откаченную транзакцию.
func (e *Editor) Redo() bool {
	if e.Buf.ReadOnly || e.groupDepth > 0 || len(e.redo) == 0 {
		return false
	}
	entry := e.redo[len(e.redo)-1]
	e.redo = e.redo[:len(e.redo)-1]
	for _, ch := range entry.changes {
		r := Range{Start: ch.start, End: e.Buf.posAfter(ch.start, ch.old)}
		e.Buf.replace(r, ch.new)
	}
	e.restore(entry.after)
	e.undo = append(e.undo, entry)
	e.sealLast()
	e.Buf.revision = entry.afterRev
	e.Buf.dirty = e.Buf.revision != e.Buf.savedRevision
	return true
}

// CanUndo — есть ли что откатывать.
func (e *Editor) CanUndo() bool {
	return !e.Buf.ReadOnly && e.groupDepth == 0 && len(e.undo) > 0
}

// CanRedo — есть ли что повторять.
func (e *Editor) CanRedo() bool {
	return !e.Buf.ReadOnly && e.groupDepth == 0 && len(e.redo) > 0
}
