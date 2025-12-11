package util

import "testing"

func TestRandomStringLength(t *testing.T) {
	const n = 32
	s := RandomString(n)
	if len(s) != n {
		t.Fatalf("expected length %d, got %d", n, len(s))
	}
}

func TestRandomStringUniqueness(t *testing.T) {
	s1 := RandomString(16)
	s2 := RandomString(16)
	if s1 == s2 {
		t.Fatalf("expected different random strings, got identical %q", s1)
	}
}
