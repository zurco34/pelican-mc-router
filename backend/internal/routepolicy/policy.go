// Package routepolicy persists operator-selected routing policy.
package routepolicy

import "errors"

var (
	ErrNotFound = errors.New("route policy not found")
	ErrConflict = errors.New("route policy revision conflict")
	ErrInvalid  = errors.New("invalid route policy")
)

// Policy is an operator-selected policy for one immutable Pelican server UUID.
// Hostname semantics and collision checks intentionally belong to the planner.
type Policy struct {
	ServerUUID      string
	PrimaryHostname string
	Aliases         []string
	Excluded        bool
	Revision        int64
}
