package pelican

import (
	"context"
	"fmt"
)

// ListNodeAllocations retrieves every allocation belonging to a Pelican node.
func (c *Client) ListNodeAllocations(
	ctx context.Context,
	nodeID int,
) ([]AllocationResource, error) {
	if nodeID <= 0 {
		return nil, fmt.Errorf("node ID must be greater than zero")
	}

	endpoint := fmt.Sprintf("/nodes/%d/allocations", nodeID)

	return listAll[AllocationResource](ctx, c, endpoint)
}
