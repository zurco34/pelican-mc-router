package pelican

import "context"

// ListEggs retrieves all eggs visible through the Pelican Application API.
func (c *Client) ListEggs(ctx context.Context) ([]EggResource, error) {
	return listAll[EggResource](ctx, c, "/eggs")
}
