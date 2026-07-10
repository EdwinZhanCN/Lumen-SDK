//go:build linux

package native

import (
	"strings"
	"testing"
)

func TestGenerateUnitContainsExecStart(t *testing.T) {
	unit := generateUnit("/usr/local/bin/lumen-hostd", []string{"serve", "--config", "/etc/lumen/hostd.yaml"})

	for _, want := range []string{
		"ExecStart=/usr/local/bin/lumen-hostd serve --config /etc/lumen/hostd.yaml",
		"Restart=on-failure",
		"WantedBy=default.target",
	} {
		if !strings.Contains(unit, want) {
			t.Errorf("generateUnit output missing %q\n---\n%s", want, unit)
		}
	}
}

func TestGenerateUnitNoArgs(t *testing.T) {
	unit := generateUnit("/usr/local/bin/lumen-hostd", nil)
	if !strings.Contains(unit, "ExecStart=/usr/local/bin/lumen-hostd\n") {
		t.Errorf("generateUnit with no args should have a bare ExecStart line:\n%s", unit)
	}
}
