package api

import (
	"time"

	"github.com/zurco34/pelican-mc-router/internal/actionhistory"
	"github.com/zurco34/pelican-mc-router/internal/operationalhistory"
	"github.com/zurco34/pelican-mc-router/internal/routepolicy"
)

type actionEventResponse struct {
	OccurredAt string `json:"occurred_at"`
	Action     string `json:"action"`
	Outcome    string `json:"outcome"`
}
type operationalEventResponse struct {
	OccurredAt string `json:"occurred_at"`
	Kind       string `json:"kind"`
	Outcome    string `json:"outcome"`
	Desired    int    `json:"desired"`
	Created    int    `json:"created"`
	Updated    int    `json:"updated"`
	Deleted    int    `json:"deleted"`
	Changed    bool   `json:"changed"`
}
type routePolicyResponse struct {
	ServerUUID      string   `json:"server_uuid"`
	PrimaryHostname string   `json:"primary_hostname"`
	Aliases         []string `json:"aliases"`
	Excluded        bool     `json:"excluded"`
	Revision        int64    `json:"revision"`
}

func actionEventsResponse(values []actionhistory.Event) []actionEventResponse {
	result := make([]actionEventResponse, 0, len(values))
	for _, v := range values {
		result = append(result, actionEventResponse{OccurredAt: v.OccurredAt.UTC().Format(time.RFC3339Nano), Action: string(v.Action), Outcome: string(v.Outcome)})
	}
	return result
}
func operationalEventsResponse(values []operationalhistory.Event) []operationalEventResponse {
	result := make([]operationalEventResponse, 0, len(values))
	for _, v := range values {
		result = append(result, operationalEventResponse{OccurredAt: v.OccurredAt.UTC().Format(time.RFC3339Nano), Kind: string(v.Kind), Outcome: string(v.Outcome), Desired: v.Desired, Created: v.Created, Updated: v.Updated, Deleted: v.Deleted, Changed: v.Changed})
	}
	return result
}
func routePolicyResponseFor(value routepolicy.Policy) routePolicyResponse {
	return routePolicyResponse{ServerUUID: value.ServerUUID, PrimaryHostname: value.PrimaryHostname, Aliases: value.Aliases, Excluded: value.Excluded, Revision: value.Revision}
}
func routePoliciesResponse(values []routepolicy.Policy) []routePolicyResponse {
	result := make([]routePolicyResponse, 0, len(values))
	for _, v := range values {
		result = append(result, routePolicyResponseFor(v))
	}
	return result
}
