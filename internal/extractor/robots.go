package extractor

import (
	"net/http"
	"strings"
)

func RobotsFromHeaders(header http.Header) RobotsDirectives {
	var out RobotsDirectives
	for _, value := range header.Values("X-Robots-Tag") {
		for _, segment := range strings.Split(value, ",") {
			segment = strings.TrimSpace(segment)
			if segment == "" { continue }

			if i := strings.IndexByte(segment, ':'); i >= 0 {
				agent := strings.ToLower(strings.TrimSpace(segment[:i]))
				if agent != "poiskbot" && agent != "*" {
					continue
				}
				segment = strings.TrimSpace(segment[i+1:])
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
