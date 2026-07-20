package pelican

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

func listAll[T any](
	ctx context.Context,
	client *Client,
	path string,
) ([]T, error) {
	const firstPage = 1

	items := make([]T, 0)
	page := firstPage

	for {
		response, err := listPage[T](
			ctx,
			client,
			path,
			page,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"pelican: list %s page %d: %w",
				path,
				page,
				err,
			)
		}

		items = append(items, response.Data...)

		totalPages := response.Meta.Pagination.TotalPages

		// Pelican responses without pagination metadata are treated
		// as single-page responses.
		if totalPages <= 1 || page >= totalPages {
			break
		}

		page++
	}

	return items, nil
}

func listPage[T any](
	ctx context.Context,
	client *Client,
	path string,
	page int,
) (*ListResponse[T], error) {
	query := url.Values{}
	query.Set("page", strconv.Itoa(page))

	endpoint := path + "?" + query.Encode()

	var response ListResponse[T]

	if err := client.do(
		ctx,
		http.MethodGet,
		endpoint,
		nil,
		&response,
	); err != nil {
		return nil, err
	}

	return &response, nil
}
