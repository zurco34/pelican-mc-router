package settings

const (
	KeyPelicanURL    = "pelican.url"
	KeyPelicanAPIKey = "pelican.api_key"
	KeyRouterDomain  = "router.domain"
)

type Settings struct {
	PelicanURL    string
	PelicanAPIKey string
	RouterDomain  string
}
