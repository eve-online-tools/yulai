package feature

import "testing"

func TestCovers(t *testing.T) {
	tests := []struct {
		name       string
		have, need []string
		want       bool
	}{
		{"superset", []string{"b", "a", "c"}, []string{"a", "b"}, true},
		{"missing scope", []string{"a"}, []string{"a", "b"}, false},
		{"nothing needed", nil, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Covers(tt.have, tt.need); got != tt.want {
				t.Errorf("Covers(%v, %v) = %v, want %v", tt.have, tt.need, got, tt.want)
			}
		})
	}
}
