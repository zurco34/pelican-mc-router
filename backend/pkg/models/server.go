package models

type Server struct {
	ID         string
	UUID       string
	Name       string
	Identifier string

	Type ServerType

	Hostname string

	Suspended bool

	NodeID int

	Metadata map[string]string
}
