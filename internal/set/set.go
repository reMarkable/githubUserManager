// Package set provides a small string-set helper used throughout the manager.
// Values are compared case-sensitively; callers are expected to normalise
// (lower-case/trim) before inserting, mirroring the original Node implementation.
package set

import "sort"

// Set is an unordered collection of unique strings.
type Set map[string]struct{}

// New builds a Set from the given values, skipping empty strings.
func New(values ...string) Set {
	s := make(Set, len(values))
	for _, v := range values {
		s.Add(v)
	}
	return s
}

// Add inserts a non-empty value into the set.
func (s Set) Add(value string) {
	if value == "" {
		return
	}
	s[value] = struct{}{}
}

// Has reports whether value is present.
func (s Set) Has(value string) bool {
	_, ok := s[value]
	return ok
}

// Len returns the number of elements.
func (s Set) Len() int { return len(s) }

// Diff returns the elements in s that are not in other.
func (s Set) Diff(other Set) Set {
	out := make(Set)
	for v := range s {
		if !other.Has(v) {
			out.Add(v)
		}
	}
	return out
}

// Union returns the elements present in either s or other.
func (s Set) Union(other Set) Set {
	out := make(Set, len(s)+len(other))
	for v := range s {
		out.Add(v)
	}
	for v := range other {
		out.Add(v)
	}
	return out
}

// Sorted returns the elements as a deterministically ordered slice, which keeps
// logging stable across runs (Go map iteration order is randomised).
func (s Set) Sorted() []string {
	out := make([]string, 0, len(s))
	for v := range s {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
