package extractor

import (
	"net/http"
	"strings"
)

func RobotsFromHeaders(header http.Header) RobotsDirectives {
	var out RobotsDirectives
	for _, value := range header.Values("X-Robots-Tag") {
		currentAgent := ""
		for _, segment := range strings.Split(value, ",") {
			segment = strings.TrimSpace(segment)
			if segment == "" { continue }

			if i := strings.IndexByte(segment, ':'); i >= 0 {
				currentAgent = strings.ToLower(strings.TrimSpace(segment[:i]))
				segment = strings.TrimSpace(segment[i+1:])
			}
			if currentAgent != "" && currentAgent != "poiskbot" && currentAgent != "*" {
				continue
			}
			applyRobots(&out, segment)
		}
	}
	return out
}

func MergeRobots(base, extra RobotsDirectives) RobotsDirectives {
	return RobotsDirectives{
		NoIndex:  base.NoIndex || extra.NoIndex,
		NoFollow: base.NoFollow || extra.NoFollow,
	}
}
