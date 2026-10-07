package gog

import "testing"

func TestVersion(t *testing.T) {
	tests := []struct {
		out  string
		want [3]int
		ok   bool
	}{
		{"v0.40.0 (a3065f53 2026-09-11T23:26:00Z)\n", [3]int{0, 40, 0}, true},
		{"v0.41.2 (deadbeef)", [3]int{0, 41, 2}, true},
		{"v0.39.9 (x)", [3]int{0, 39, 9}, false},
		{"v1.0.0", [3]int{1, 0, 0}, true},
		{"0.9.100", [3]int{0, 9, 100}, false},
	}
	for _, tt := range tests {
		v, err := ParseVersion(tt.out)
		if err != nil || v != tt.want {
			t.Errorf("ParseVersion(%q) = %v, %v; want %v", tt.out, v, err, tt.want)
		}
		if got := AtLeast(v, MinVersion); got != tt.ok {
			t.Errorf("AtLeast(%v) = %v, want %v", v, got, tt.ok)
		}
	}
	if _, err := ParseVersion("garbage"); err == nil {
		t.Error("ParseVersion(garbage) should fail")
	}
}
