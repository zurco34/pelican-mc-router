package pelican

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/zurco34/pelican-mc-router/internal/retry"
)

const userAgent = "pelican-mc-router"

func (c *Client) do(
	ctx context.Context,
	method string,
	path string,
	body any,
	out any,
) error {
	endpoint, err := c.buildURL(path)
	if err != nil {
		return err
	}

	var requestBody io.Reader

	if body != nil {
		var buffer bytes.Buffer

		if err := json.NewEncoder(&buffer).Encode(body); err != nil {
			return fmt.Errorf("pelican: encode request body: %w", err)
		}

		requestBody = &buffer
	}

	req, err := http.NewRequestWithContext(
		ctx,
		method,
		endpoint,
		requestBody,
	)
	if err != nil {
		return fmt.Errorf("pelican: create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	req.Header.Set("User-Agent", userAgent)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.doRequest(ctx, method, req)
	if err != nil {
		return fmt.Errorf("pelican: execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK ||
		resp.StatusCode >= http.StatusMultipleChoices {
		return responseError(resp.StatusCode)
	}

	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("pelican: decode response: %w", err)
	}

	return nil
}

func (c *Client) doRequest(
	ctx context.Context,
	method string,
	req *http.Request,
) (*http.Response, error) {
	if method != http.MethodGet {
		return c.httpClient.Do(req)
	}

	return retry.Do(ctx, c.retry, func() (*http.Response, error) {
		return c.httpClient.Do(req)
	})
}

func (c *Client) buildURL(path string) (string, error) {
	baseURL, err := url.Parse(c.cfg.BaseURL + "/")
	if err != nil {
		return "", fmt.Errorf("pelican: parse base URL: %w", err)
	}

	relativeURL, err := url.Parse(strings.TrimPrefix(path, "/"))
	if err != nil {
		return "", fmt.Errorf("pelican: parse request path: %w", err)
	}

	return baseURL.ResolveReference(relativeURL).String(), nil
}

func responseError(statusCode int) error {
	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf(
			"%w: HTTP status %d",
			ErrUnauthorized,
			statusCode,
		)

	case http.StatusNotFound:
		return fmt.Errorf(
			"%w: HTTP status %d",
			ErrNotFound,
			statusCode,
		)

	default:
		return fmt.Errorf(
			"%w: HTTP status %d",
			ErrUnexpected,
			statusCode,
		)
	}
}
