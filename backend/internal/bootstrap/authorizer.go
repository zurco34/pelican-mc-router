// Package bootstrap authorizes one-time setup with a mounted bearer token.
package bootstrap

import (
	"bytes"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
)

var ErrUnauthorized = errors.New("bootstrap authorization failed")

type SecretReader interface {
	Read(name string) ([]byte, error)
}

type Authorizer struct {
	reader    SecretReader
	tokenName string
}

func New(reader SecretReader, tokenName string) (*Authorizer, error) {
	if reader == nil || strings.TrimSpace(tokenName) == "" {
		return nil, ErrUnauthorized
	}
	return &Authorizer{reader: reader, tokenName: tokenName}, nil
}

func (a *Authorizer) Authorize(request *http.Request) error {
	if a == nil || request == nil {
		return ErrUnauthorized
	}

	provided, ok := bearerToken(request.Header.Get("Authorization"))
	if !ok {
		return ErrUnauthorized
	}

	expected, err := a.reader.Read(a.tokenName)
	if err != nil {
		return ErrUnauthorized
	}
	defer clear(expected)

	if expected = bytes.TrimSpace(expected); len(expected) == 0 || subtle.ConstantTimeCompare(expected, []byte(provided)) != 1 {
		return ErrUnauthorized
	}
	return nil
}

func bearerToken(value string) (string, bool) {
	parts := strings.Fields(value)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}
