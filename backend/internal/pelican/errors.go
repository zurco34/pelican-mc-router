package pelican

import "errors"

var (
	ErrInvalidBaseURL = errors.New("pelican: invalid base URL")
	ErrMissingAPIKey  = errors.New("pelican: API key is required")

	ErrUnauthorized = errors.New("pelican: unauthorized")
	ErrNotFound     = errors.New("pelican: resource not found")
	ErrUnexpected   = errors.New("pelican: unexpected response")
)
