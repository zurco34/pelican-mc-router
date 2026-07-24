package router

import (
	"errors"
	"fmt"
	"strings"
)

type Engine string

const (
	EngineMCRouter Engine = "mc-router"
	EngineInfrared Engine = "infrared"
)

var ErrUnsupportedEngine = errors.New(
	"router: unsupported engine",
)

func ParseEngine(value string) (Engine, error) {
	engine := Engine(strings.TrimSpace(value))

	switch engine {
	case EngineMCRouter, EngineInfrared:
		return engine, nil
	default:
		return "", fmt.Errorf(
			"%w: %q",
			ErrUnsupportedEngine,
			value,
		)
	}
}
