package ui

import (
	"testing"
)

func TestLayoutSizes(t *testing.T) {
	for _, tc := range []struct{ w, h int }{{40, 10}, {80, 24}, {160, 50}, {20, 5}, {1, 1}} {
		m := testModel(t)
		setSize(m, tc.w, tc.h)
		typeText(m, "привет 😀\n\tвторая строка - длинная-предлинная-предлинная-предлинная")
		viewLines(t, m)
		lay := m.layout()
		if lay.text.w < 0 || lay.text.h < 0 {
			t.Fatalf("%dx%d: отрицательная текстовая зона: %+v", tc.w, tc.h, lay.text)
		}
		if lay.status.y != tc.h-1 && tc.h > 0 {
			t.Fatalf("%dx%d: статус не в последней строке: %d", tc.w, tc.h, lay.status.y)
		}
	}
}

func TestGutterHiddenOnNarrowScreen(t *testing.T) {
	m := testModel(t)
	setSize(m, 39, 10)
	if lay := m.layout(); lay.gutterW != 0 {
		t.Fatalf("гаттер должен скрываться: %d", lay.gutterW)
	}
	setSize(m, 40, 10)
	if lay := m.layout(); lay.gutterW == 0 {
		t.Fatal("на 40 колонках гаттер должен быть")
	}
}

func TestTabsBarManyTabs(t *testing.T) {
	m := testModel(t)
	for i := 0; i < 24; i++ {
		m.newEmptyTab()
	}
	setSize(m, 80, 24)
	m.active = len(m.tabs) - 1
	viewLines(t, m)
	st := m.tabBarState()
	found := false
	for _, h := range st.hits {
		if h.idx == m.active {
			found = true
		}
	}
	if !found {
		t.Fatal("активная вкладка не попала в полосу")
	}
}

func TestSingleTabHidesBar(t *testing.T) {
	m := testModel(t)
	setSize(m, 80, 24)
	if lay := m.layout(); lay.tabs.h != 0 {
		t.Fatal("полоса вкладок должна быть скрыта при одной вкладке")
	}
}
