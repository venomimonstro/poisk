package release

import (
	"encoding/hex"
	"regexp"
	"strings"
)

var (
	imageRefPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/@-]{0,511}$`)
	versionPattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
	buildPattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
)

func validReleaseVersion(v string) bool { return versionPattern.MatchString(strings.TrimSpace(v)) }
func validBuildSHA(v string) bool { return buildPattern.MatchString(strings.TrimSpace(v)) }
func validImageRef(v string) bool { return imageRefPattern.MatchString(strings.TrimSpace(v)) }
func validSHA256(v string) bool {
	v = strings.TrimSpace(v)
	if len(v) != 64 { return false }
	b, err := hex.DecodeString(v)
	return err == nil && len(b) == 32
}
