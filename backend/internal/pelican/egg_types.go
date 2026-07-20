package pelican

type EggResource struct {
	Object     string        `json:"object"`
	Attributes EggAttributes `json:"attributes"`
}

type EggAttributes struct {
	ID          int      `json:"id"`
	UUID        string   `json:"uuid"`
	Name        string   `json:"name"`
	Author      string   `json:"author"`
	Description string   `json:"description"`
	Image       *string  `json:"image"`
	Features    []string `json:"features"`
	Tags        []string `json:"tags"`

	DockerImage string `json:"docker_image"`

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}
