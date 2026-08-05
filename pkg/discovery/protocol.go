package discovery

import (
	"regexp"
	"strconv"
	"strings"
)

// protocolVersionRe matches the accepted data-plane version forms: "1",
// "1.0", "1.0.0", and semver prerelease tails like "1.0.0-beta.1".
var protocolVersionRe = regexp.MustCompile(`^([0-9]+)(\.[0-9]+)*(\-[0-9A-Za-z.-]+)?$`)

// ParseProtocolMajor parses a data-plane protocol version into its major
// component. Accepted forms: "1", "1.0", "1.0.0", "v1.0.0". Returns ok=false
// for empty or unparsable values.
func ParseProtocolMajor(version string) (int, bool) {
	version = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(version, "v"), "V"))
	matches := protocolVersionRe.FindStringSubmatch(version)
	if matches == nil {
		return 0, false
	}
	major, err := strconv.Atoi(matches[1])
	if err != nil || major < 0 {
		return 0, false
	}
	return major, true
}
