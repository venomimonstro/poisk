package webmaster

import "errors"

var (
	ErrInvalidInput = errors.New("invalid webmaster input")
	ErrSiteBlocked  = errors.New("site is blocked by crawl policy")
)
