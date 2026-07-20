package pelican

type NodeResource struct {
	Object     string         `json:"object"`
	Attributes NodeAttributes `json:"attributes"`
}

type NodeAttributes struct {
	ID                 int     `json:"id"`
	UUID               string  `json:"uuid"`
	Public             bool    `json:"public"`
	Name               string  `json:"name"`
	Description        *string `json:"description"`
	LocationID         int     `json:"location_id"`
	FQDN               string  `json:"fqdn"`
	Scheme             string  `json:"scheme"`
	BehindProxy        bool    `json:"behind_proxy"`
	MaintenanceMode    bool    `json:"maintenance_mode"`
	Memory             int     `json:"memory"`
	MemoryOverallocate int     `json:"memory_overallocate"`
	Disk               int     `json:"disk"`
	DiskOverallocate   int     `json:"disk_overallocate"`
	UploadSize         int     `json:"upload_size"`
	DaemonListen       int     `json:"daemon_listen"`
	DaemonSFTP         int     `json:"daemon_sftp"`
	DaemonBase         string  `json:"daemon_base"`
}
