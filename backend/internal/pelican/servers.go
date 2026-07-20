package pelican

import "context"

// ListServers retrieves every server visible through the Pelican
// Application API, following all pagination pages.
func (c *Client) ListServers(
	ctx context.Context,
) ([]ServerResource, error) {
	return listAll[ServerResource](ctx, c, "/servers")
}
