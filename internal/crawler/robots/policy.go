package robots

import (
	"errors"
	"net/url"
	"time"

	"github.com/temoto/robotstxt"
)

const UserAgent = "PoiskBot"

var ErrTooLarge = errors.New("robots.txt exceeds size limit")

type Policy struct {
	group      *robotstxt.Group
	Sitemaps   []string
	CrawlDelay time.Duration
}

func Parse(statusCode int, body []byte, maxBytes int) (Policy, error) {
	if maxBytes <= 0 {
		return Policy{}, errors.New("max robots bytes must be positive")
	}
	if len(body) > maxBytes {
		return Policy{}, ErrTooLarge
	}
	data, err := robotstxt.FromStatusAndBytes(statusCode, body)
	if err != nil {
		return Policy{}, err
	}
	group := data.FindGroup(UserAgent)
	return Policy{
		group:      group,
		Sitemaps:   append([]string(nil), data.Sitemaps...),
		CrawlDelay: group.CrawlDelay,
	}, nil
}

func (p Policy) Allowed(rawURL string) bool {
	if p.group == nil {
		return false
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}
	return p.group.Test(path)
}
