package webmaster

import (
	"context"
	"errors"
	"fmt"
	"strings"

	crawlersecurity "github.com/venomimonstro/poisk/internal/crawler/security"
)

var ErrInvalidSiteOrigin = errors.New("invalid site origin")

type SiteOrigin struct {
	Origin string
	Host   string
}

func ValidateSiteOrigin(ctx context.Context, validator crawlersecurity.Validator, raw string) (SiteOrigin, error) {
	target, err := validator.Validate(ctx, strings.TrimSpace(raw))
	if err != nil { return SiteOrigin{}, fmt.Errorf("%w: %v", ErrInvalidSiteOrigin, err) }
	u := target.URL
	if u.Path != "" && u.Path != "/" { return SiteOrigin{}, ErrInvalidSiteOrigin }
	if u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" { return SiteOrigin{}, ErrInvalidSiteOrigin }
	origin := u.Scheme + "://" + target.Host
	if (u.Scheme == "http" && target.Port != 80) || (u.Scheme == "https" && target.Port != 443) {
		origin += fmt.Sprintf(":%d", target.Port)
	}
	return SiteOrigin{Origin: origin, Host: target.Host}, nil
}
