package extractor

import (
	"net/http"
	"testing"
)

func TestRobotsFromHeadersScopesPoiskBot(t *testing.T) {
	h := make(http.Header)
	h.Add("X-Robots-Tag", "googlebot: noindex")
	h.Add("X-Robots-Tag", "PoiskBot: nofollow, noindex")
	got := RobotsFromHeaders(h)
	if !got.NoIndex || !got.NoFollow { t.Fatalf("robots=%+v", got) }
}

func TestRobotsFromHeadersKeepsForeignAgentScope(t *testing.T) {
	h := make(http.Header)
	h.Add("X-Robots-Tag", "googlebot: noindex, nofollow")
	got := RobotsFromHeaders(h)
	if got.NoIndex || got.NoFollow { t.Fatalf("foreign directives leaked into PoiskBot policy: %+v", got) }
}

func TestRobotsFromHeadersGenericAndMerge(t *testing.T) {
	h := make(http.Header)
	h.Add("X-Robots-Tag", "noindex")
	fromHeader := RobotsFromHeaders(h)
	merged := MergeRobots(RobotsDirectives{NoFollow: true}, fromHeader)
	if !merged.NoIndex || !merged.NoFollow { t.Fatalf("robots=%+v", merged) }
}
