package set

import (
	"reflect"
	"testing"
)

func TestNewSkipsEmpty(t *testing.T) {
	s := New("a", "", "b", "a")
	if s.Len() != 2 {
		t.Errorf("Len = %d, want 2", s.Len())
	}
	if s.Has("") {
		t.Error("empty string should not be present")
	}
}

func TestDiff(t *testing.T) {
	a := New("a", "b", "c")
	b := New("b")
	if got := a.Diff(b).Sorted(); !reflect.DeepEqual(got, []string{"a", "c"}) {
		t.Errorf("Diff = %v", got)
	}
}

func TestUnion(t *testing.T) {
	a := New("a", "b")
	b := New("b", "c")
	if got := a.Union(b).Sorted(); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Errorf("Union = %v", got)
	}
}

func TestSortedDeterministic(t *testing.T) {
	s := New("c", "a", "b")
	if got := s.Sorted(); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Errorf("Sorted = %v", got)
	}
}
