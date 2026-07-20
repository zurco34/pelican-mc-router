package pelican

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// ListServers retrieves every server visible through the Pelican
// Application API, following all pagination pages.
func (c *Client) ListServers(
	ctx context.Context,
) ([]ServerResource, error) {
	const firstPage = 1

	servers := make([]ServerResource, 0)
	page := firstPage

	for {
		response, err := c.listServersPage(ctx, page)
		if err != nil {
			return nil, fmt.Errorf(
				"pelican: list servers page %d: %w",
				page,
				err,
			)
		}

		servers = append(servers, response.Data...)

		totalPages := response.Meta.Pagination.TotalPages

		// A missing pagination object is treated as a single-page response.
		if totalPages <= 1 || page >= totalPages {
			break
		}

		page++
	}

	return servers, nil
}

func (c *Client) listServersPage(
	ctx context.Context,
	page int,
) (*ListResponse[ServerResource], error) {
	query := url.Values{}
	query.Set("page", strconv.Itoa(page))

	path := "/servers?" + query.Encode()

	var response ListResponse[ServerResource]

	if err := c.do(
		ctx,
		http.MethodGet,
		path,
		nil,
		&response,
	); err != nil {
		return nil, err
	}

	return &response, nil
}
