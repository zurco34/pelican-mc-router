package pelican

type ListResponse[T any] struct {
	Object string       `json:"object"`
	Data   []T          `json:"data"`
	Meta   ResponseMeta `json:"meta"`
}

type ResponseMeta struct {
	Pagination Pagination `json:"pagination"`
}

type Pagination struct {
	Total       int `json:"total"`
	Count       int `json:"count"`
	PerPage     int `json:"per_page"`
	CurrentPage int `json:"current_page"`
	TotalPages  int `json:"total_pages"`
}

type ServerResource struct {
	Object     string           `json:"object"`
	Attributes ServerAttributes `json:"attributes"`
}

type ServerAttributes struct {
	ID          int     `json:"id"`
	UUID        string  `json:"uuid"`
	Identifier  string  `json:"identifier"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Status      *string `json:"status"`

	Suspended bool `json:"suspended"`

	Node int `json:"node"`
	Egg  int `json:"egg"`

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}
