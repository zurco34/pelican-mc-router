package settings

const (
	KeyPelicanURL        = "pelican.url"
	KeyPelicanAPIKey     = "pelican.api_key"
	KeyPelicanSecretName = "pelican.secret_name"
	KeyRouterDomain      = "router.domain"
)

type Settings struct {
	PelicanURL        string
	PelicanAPIKey     string
	PelicanSecretName string
	RouterDomain      string
}
