package router

import "context"

// ReconciliationResult contains bounded, backend-safe reconciliation facts.
type ReconciliationResult struct {
	Desired int
	Created int
	Updated int
	Deleted int
	Changed bool
}

type DiagnosticsRouteController interface {
	ReconcileWithResult(context.Context, []Route) (ReconciliationResult, error)
}
