package robots

import "testing"

func TestPolicyForPoiskBotAndSitemaps(t *testing.T) {
	body := []byte("User-agent: *\nDisallow: /private\n\nUser-agent: PoiskBot\nDisallow: /secret\nAllow: /secret/public\nCrawl-delay: 2\nSitemap: https://example.com/sitemap.xml\n")
	p, err := Parse(200, body, 1024)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if !p.Allowed("https://example.com/") {
		t.Fatal("root should be allowed")
	}
	if p.Allowed("https://example.com/secret") {
		t.Fatal("/secret should be disallowed")
	}
	if !p.Allowed("https://example.com/secret/public") {
		t.Fatal("explicit allow should win")
	}
	if len(p.Sitemaps) != 1 || p.Sitemaps[0] != "https://example.com/sitemap.xml" {
		t.Fatalf("unexpected sitemaps: %#v", p.Sitemaps)
	}
	if p.CrawlDelay.Seconds() != 2 {
		t.Fatalf("unexpected crawl delay: %v", p.CrawlDelay)
	}
}

func TestRobotsStatusSemantics(t *testing.T) {
	allow, err := Parse(404, nil, 1024)
	if err != nil {
		t.Fatalf("404 Parse() = %v", err)
	}
	if !allow.Allowed("https://example.com/anything") {
		t.Fatal("404 robots should allow crawling")
	}

	deny, err := Parse(503, nil, 1024)
	if err != nil {
		t.Fatalf("503 Parse() = %v", err)
	}
	if deny.Allowed("https://example.com/") {
		t.Fatal("503 robots should fail closed")
	}
}

func TestRobotsSizeLimit(t *testing.T) {
	if _, err := Parse(200, []byte("12345"), 4); err == nil {
		t.Fatal("expected size limit error")
	}
}
