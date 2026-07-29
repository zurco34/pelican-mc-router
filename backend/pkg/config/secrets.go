package config

// SecretsConfig defines the mounted directory used by bounded secret readers.
// Secret values are never configured through this structure.
type SecretsConfig struct {
	Directory          string `mapstructure:"directory"`
	BootstrapTokenName string `mapstructure:"bootstrap_token_name"`
}
