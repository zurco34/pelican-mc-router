package models

type MinecraftServer struct {
	ID           int
	UUID         string
	Identifier   string
	Name         string
	NodeID       int
	EggID        int
	AllocationID int
	Suspended    bool
	Status       *string
}
