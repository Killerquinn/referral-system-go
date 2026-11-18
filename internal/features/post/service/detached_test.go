package service

import (
	"slices"
	"testing"
)

func TestDetached(t *testing.T) {
	tests := []struct {
		name          string
		before, after []string
		want          []string
	}{
		{"nothing attached before", nil, []string{"a"}, nil},
		{"all removed", []string{"a", "b"}, []string{}, []string{"a", "b"}},
		{"only reordered", []string{"a", "b"}, []string{"b", "a"}, nil},
		{"one removed, one added", []string{"a", "b"}, []string{"b", "c"}, []string{"a"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := detached(tt.before, tt.after); !slices.Equal(got, tt.want) {
				t.Fatalf("detached(%v, %v) = %v, want %v", tt.before, tt.after, got, tt.want)
			}
		})
	}
}
