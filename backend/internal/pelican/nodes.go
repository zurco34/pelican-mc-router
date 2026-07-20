package pelican

import "context"

// ListNodes retrieves every node visible through the Pelican
// Application API, following all pagination pages.
func (c *Client) ListNodes(
	ctx context.Context,
) ([]NodeResource, error) {
	return listAll[NodeResource](ctx, c, "/nodes")
}
