package mcrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/zurco34/pelican-mc-router/internal/retry"
)

const (
	defaultHTTPTimeout = 15 * time.Second
	maxResponseSize    = 1 << 20
)

type ClientConfig struct {
	BaseURL    string
	HTTPClient *http.Client
	Retry      retry.Config
}

type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
	retry      retry.Config
}

type routeResponse struct {
	Backend       string `json:"backend"`
	ScalingTarget string `json:"scalingTarget"`
}

type createRouteRequest struct {
	ServerAddress string `json:"serverAddress"`
	Backend       string `json:"backend"`
}

func NewClient(cfg ClientConfig) (*Client, error) {
	rawBaseURL := strings.TrimSpace(cfg.BaseURL)
	if rawBaseURL == "" {
		return nil, fmt.Errorf(
			"mcrouter: base URL is required",
		)
	}

	baseURL, err := url.Parse(rawBaseURL)
	if err != nil {
		return nil, fmt.Errorf(
			"mcrouter: parse base URL: %w",
			err,
		)
	}

	if baseURL.Scheme != "http" &&
		baseURL.Scheme != "https" {
		return nil, fmt.Errorf(
			"mcrouter: base URL scheme must be HTTP or HTTPS",
		)
	}

	if baseURL.Host == "" {
		return nil, fmt.Errorf(
			"mcrouter: base URL host is required",
		)
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: defaultHTTPTimeout,
		}
	}

	return &Client{
		baseURL:    baseURL,
		httpClient: httpClient,
		retry:      cfg.Retry,
	}, nil
}

func (c *Client) ListRoutes(
	ctx context.Context,
) (map[string]string, error) {
	requestURL := *c.baseURL
	requestURL.Path = path.Join(
		requestURL.Path,
		"routes",
	)

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		requestURL.String(),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"mcrouter: create list routes request: %w",
			err,
		)
	}

	response, err := retry.Do(ctx, c.retry, func() (*http.Response, error) {
		return c.httpClient.Do(request)
	})
	if err != nil {
		return nil, fmt.Errorf(
			"mcrouter: list routes request: %w",
			err,
		)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK ||
		response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(
			io.Discard,
			io.LimitReader(response.Body, maxResponseSize),
		)

		return nil, fmt.Errorf(
			"mcrouter: list routes returned status %s",
			response.Status,
		)
	}

	var responseRoutes map[string]routeResponse

	decoder := json.NewDecoder(
		io.LimitReader(response.Body, maxResponseSize),
	)

	if err := decoder.Decode(&responseRoutes); err != nil {
		return nil, fmt.Errorf(
			"mcrouter: decode list routes response: %w",
			err,
		)
	}

	routes := make(map[string]string, len(responseRoutes))
	for hostname, route := range responseRoutes {
		routes[hostname] = route.Backend
	}

	return routes, nil
}

func (c *Client) CreateRoute(
	ctx context.Context,
	hostname string,
	backend string,
) error {
	requestBody, err := json.Marshal(createRouteRequest{
		ServerAddress: hostname,
		Backend:       backend,
	})
	if err != nil {
		return fmt.Errorf(
			"mcrouter: encode create route request: %w",
			err,
		)
	}

	requestURL := *c.baseURL
	requestURL.Path = path.Join(
		requestURL.Path,
		"routes",
	)

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		requestURL.String(),
		bytes.NewReader(requestBody),
	)
	if err != nil {
		return fmt.Errorf(
			"mcrouter: create route request: %w",
			err,
		)
	}

	request.Header.Set(
		"Content-Type",
		"application/json",
	)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf(
			"mcrouter: create route request: %w",
			err,
		)
	}
	defer response.Body.Close()

	_, _ = io.Copy(
		io.Discard,
		io.LimitReader(response.Body, maxResponseSize),
	)

	if response.StatusCode < http.StatusOK ||
		response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf(
			"mcrouter: create route returned status %s",
			response.Status,
		)
	}

	return nil
}

func (c *Client) DeleteRoute(
	ctx context.Context,
	hostname string,
) error {
	requestURL := *c.baseURL
	requestURL.Path = path.Join(
		requestURL.Path,
		"routes",
		hostname,
	)

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodDelete,
		requestURL.String(),
		nil,
	)
	if err != nil {
		return fmt.Errorf(
			"mcrouter: create delete route request: %w",
			err,
		)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf(
			"mcrouter: delete route request: %w",
			err,
		)
	}
	defer response.Body.Close()

	_, _ = io.Copy(
		io.Discard,
		io.LimitReader(response.Body, maxResponseSize),
	)

	if response.StatusCode < http.StatusOK ||
		response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf(
			"mcrouter: delete route returned status %s",
			response.Status,
		)
	}

	return nil
}
