package urlnorm

import (
	"errors"
	"net"
	"net/url"
	"strings"

	"golang.org/x/net/idna"
)

var (
	ErrUnsupportedScheme = errors.New("unsupported URL scheme")
	ErrMissingHost       = errors.New("URL host is required")
	ErrUserInfo          = errors.New("userinfo in URL is not allowed")
)

var trackingKeys = map[string]struct{}{
	"gclid":     {},
	"fbclid":    {},
	"yclid":     {},
	"_openstat": {},
}

func Normalize(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", ErrUnsupportedScheme
	}
	if u.User != nil {
		return "", ErrUserInfo
	}
	if u.Hostname() == "" {
		return "", ErrMissingHost
	}

	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	host, err = idna.Lookup.ToASCII(host)
	if err != nil {
		return "", err
	}
	port := u.Port()
	if (u.Scheme == "http" && port == "80") || (u.Scheme == "https" && port == "443") {
		port = ""
	}
	if port != "" {
		u.Host = net.JoinHostPort(host, port)
	} else {
		u.Host = host
	}

	if u.Path == "" {
		u.Path = "/"
	}
	u.Fragment = ""

	q := u.Query()
	for key := range q {
		lower := strings.ToLower(key)
		if strings.HasPrefix(lower, "utm_") {
			q.Del(key)
			continue
		}
		if _, ok := trackingKeys[lower]; ok {
			q.Del(key)
		}
	}
	u.RawQuery = q.Encode()

	return u.String(), nil
}
