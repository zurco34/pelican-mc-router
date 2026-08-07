package api

import (
	"time"

	"github.com/zurco34/pelican-mc-router/internal/actionhistory"
	"github.com/zurco34/pelican-mc-router/internal/operationalhistory"
	"github.com/zurco34/pelican-mc-router/internal/routepolicy"
	routing "github.com/zurco34/pelican-mc-router/internal/router"
	"github.com/zurco34/pelican-mc-router/pkg/models"
)

type actionEventResponse struct {
	OccurredAt string `json:"occurred_at"`
	Action     string `json:"action"`
	Outcome    string `json:"outcome"`
}

type serverResponse struct {
	ID           int     `json:"id"`
	UUID         string  `json:"uuid"`
	Identifier   string  `json:"identifier"`
	Name         string  `json:"name"`
	NodeID       int     `json:"node_id"`
	EggID        int     `json:"egg_id"`
	AllocationID int     `json:"allocation_id"`
	BackendIP    string  `json:"backend_ip"`
	BackendPort  int     `json:"backend_port"`
	Suspended    bool    `json:"suspended"`
	Status       *string `json:"status"`
}
type backendResponse struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}
type routeResponse struct {
	ServerID string          `json:"server_id"`
	Hostname string          `json:"hostname"`
	Backend  backendResponse `json:"backend"`
}
type serversResponse struct {
	Servers []serverResponse `json:"servers"`
}
type routesResponse struct {
	Routes []routeResponse `json:"routes"`
}
type setupStatusResponse struct {
	Completed bool `json:"completed"`
}
type readinessResponse struct {
	Ready  bool   `json:"ready"`
	Reason string `json:"reason"`
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

func serversResponseFor(values []models.MinecraftServer) []serverResponse {
	result := make([]serverResponse, 0, len(values))
	for _, v := range values {
		result = append(result, serverResponse{v.ID, v.UUID, v.Identifier, v.Name, v.NodeID, v.EggID, v.AllocationID, v.BackendIP, v.BackendPort, v.Suspended, v.Status})
	}
	return result
}
func routesResponseFor(values []routing.Route) []routeResponse {
	result := make([]routeResponse, 0, len(values))
	for _, v := range values {
		result = append(result, routeResponse{ServerID: v.ServerID, Hostname: v.Hostname, Backend: backendResponse{Host: v.Backend.Host, Port: v.Backend.Port}})
	}
	return result
}
