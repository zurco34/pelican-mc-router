// Package buildinfo exposes immutable build metadata.
package buildinfo

// These values are replaced by release builds with -ldflags.
var (
	Version  = "dev"
	Revision = "unknown"
)

type Info struct {
	Version  string `json:"version"`
	Revision string `json:"revision"`
}

func Current() Info {
	return Info{Version: Version, Revision: Revision}
}
