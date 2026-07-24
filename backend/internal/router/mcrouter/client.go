package mcrouter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

const (
	defaultHTTPTimeout = 15 * time.Second
	maxResponseSize    = 1 << 20
)

type ClientConfig struct {
	BaseURL    string
	HTTPClient *http.Client
}

type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
}

type routeResponse struct {
	Backend       string `json:"backend"`
	ScalingTarget string `json:"scalingTarget"`
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

	response, err := c.httpClient.Do(request)
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
