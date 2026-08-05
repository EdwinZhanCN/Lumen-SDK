package discovery

import "testing"

func TestParseProtocolMajor(t *testing.T) {
	tests := []struct {
		version string
		want    int
		ok      bool
	}{
		{"1", 1, true},
		{"1.0", 1, true},
		{"1.0.0", 1, true},
		{"v1.0.0", 1, true},
		{"V2.3.4", 2, true},
		{" 1.0 ", 1, true},
		{"0", 0, true},
		{"10.0", 10, true},
		{"2", 2, true},
		{"1.0.0-rc1", 1, true},
		{"", 0, false},
		{"  ", 0, false},
		{"v", 0, false},
		{"one", 0, false},
		{"1.x", 0, false},
		{"-1", 0, false},
		{"1.0.0-beta", 1, true},
	}
	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got, ok := ParseProtocolMajor(tt.version)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("ParseProtocolMajor(%q) = (%d, %v), want (%d, %v)", tt.version, got, ok, tt.want, tt.ok)
			}
		})
	}
}
