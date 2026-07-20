package pelican

import "errors"

var (
	ErrUnauthorized = errors.New("pelican: unauthorized")
	ErrNotFound     = errors.New("pelican: resource not found")
	ErrUnexpected   = errors.New("pelican: unexpected response")
)
