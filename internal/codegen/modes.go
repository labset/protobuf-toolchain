package codegen

import (
	"fmt"
	"strings"
)

func GeneratorForMode(raw string) (Generator, error) {
	p := parseParams(raw)
	switch p.mode {
	case "echo":
		return &echoGenerator{}, nil
	default:
		return nil, fmt.Errorf("unknown mode %q", p.mode)
	}
}

// params holds parsed plugin parameters.
type params struct {
	mode string
}

func parseParams(raw string) params {
	p := params{}

	for _, param := range strings.Split(raw, ",") {
		key, value, ok := strings.Cut(param, "=")
		if !ok {
			continue
		}

		switch strings.TrimSpace(key) {
		case "mode":
			p.mode = strings.TrimSpace(value)
		}
	}

	return p
}
