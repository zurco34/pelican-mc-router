package models

import "time"

type Inventory struct {
	GeneratedAt time.Time

	Servers []MinecraftServer
	Routes  []Route
}
