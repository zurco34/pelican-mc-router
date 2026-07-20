package pelican

type AllocationResource struct {
	Object     string               `json:"object"`
	Attributes AllocationAttributes `json:"attributes"`
}

type AllocationAttributes struct {
	ID       int     `json:"id"`
	IP       string  `json:"ip"`
	Alias    *string `json:"alias"`
	Port     int     `json:"port"`
	Notes    *string `json:"notes"`
	Assigned bool    `json:"assigned"`
}
