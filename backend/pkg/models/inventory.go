package models

import "time"

type Inventory struct {
	GeneratedAt time.Time

	Servers []Server
	Routes  []Route
}
