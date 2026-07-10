//go:build windows

package native

import "testing"

func TestGenerateTaskRunCommandQuotesExecPath(t *testing.T) {
	got := generateTaskRunCommand(`C:\Program Files\Lumen\lumen-hostd.exe`, []string{"serve"})
	want := `"C:\Program Files\Lumen\lumen-hostd.exe" serve`
	if got != want {
		t.Errorf("generateTaskRunCommand() = %q, want %q", got, want)
	}
}

func TestGenerateTaskRunCommandNoArgs(t *testing.T) {
	got := generateTaskRunCommand(`C:\lumen-hostd.exe`, nil)
	want := `"C:\lumen-hostd.exe"`
	if got != want {
		t.Errorf("generateTaskRunCommand() = %q, want %q", got, want)
	}
}
