//go:build darwin

package native

import (
	"strings"
	"testing"
)

func TestGeneratePlistContainsExecAndArgs(t *testing.T) {
	xml := generatePlist("com.example.test", "/usr/local/bin/lumen-hostd", []string{"serve"}, "/tmp/out.log", "/tmp/err.log")

	for _, want := range []string{
		"<string>com.example.test</string>",
		"<string>/usr/local/bin/lumen-hostd</string>",
		"<string>serve</string>",
		"<string>/tmp/out.log</string>",
		"<string>/tmp/err.log</string>",
		"<key>RunAtLoad</key>",
		"<true/>",
		"<key>KeepAlive</key>",
	} {
		if !strings.Contains(xml, want) {
			t.Errorf("generatePlist output missing %q\n---\n%s", want, xml)
		}
	}
}

func TestGeneratePlistEscapesXML(t *testing.T) {
	xml := generatePlist("label", `/path/with "quotes" & <brackets>`, nil, "/out.log", "/err.log")
	if strings.Contains(xml, `"quotes"`) || strings.Contains(xml, "<brackets>") {
		t.Errorf("generatePlist did not escape special XML characters:\n%s", xml)
	}
	if !strings.Contains(xml, "&quot;quotes&quot;") || !strings.Contains(xml, "&lt;brackets&gt;") {
		t.Errorf("generatePlist escaped output missing expected entities:\n%s", xml)
	}
}
