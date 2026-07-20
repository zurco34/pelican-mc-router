package pelican

import (
	"context"
	"net/http"
)

func (c *Client) do(
	ctx context.Context,
	method string,
	path string,
	body any,
	out any,
) error {

	req, err := http.NewRequestWithContext(
		ctx,
		method,
		c.cfg.BaseURL+path,
		nil,
	)
	if err != nil {
		return err
	}

	_ = req

	return nil
}
