package models

type MinecraftServer struct {
	ID           int    `json:"id"`
	UUID         string `json:"uuid"`
	Identifier   string `json:"identifier"`
	Name         string `json:"name"`
	NodeID       int    `json:"node_id"`
	EggID        int    `json:"egg_id"`
	AllocationID int    `json:"allocation_id"`

	BackendIP   string `json:"backend_ip"`
	BackendPort int    `json:"backend_port"`

	Suspended bool    `json:"suspended"`
	Status    *string `json:"status"`
}
