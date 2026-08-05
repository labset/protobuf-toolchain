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
	case "proto-service":
		return &protoServiceGenerator{}, nil
	case "go-sqlc-atlas":
		if err := validateMigrationFormat(p.migration); err != nil {
			return nil, err
		}
		return &goSqlcAtlasGenerator{migration: p.migration, schema: p.schema}, nil
	default:
		return nil, fmt.Errorf("unknown mode %q", p.mode)
	}
}

// params holds parsed plugin parameters.
type params struct {
	mode string
	// migration selects the Atlas migration directory format for the
	// go-sqlc-atlas mode (goose, flyway, ...). Empty means declarative schema
	// management (atlas schema apply) with no migrations directory.
	migration string
	// schema overrides the Postgres schema the go-sqlc-atlas tables live under.
	// Empty derives it per package from the proto package name.
	schema string
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
		case "migration":
			p.migration = strings.TrimSpace(value)
		case "schema":
			p.schema = strings.TrimSpace(value)
		}
	}

	return p
}

// migrationFormats are the Atlas-supported migration directory formats. Atlas's
// own default format is selected by passing "atlas".
var migrationFormats = map[string]bool{
	"atlas":          true,
	"golang-migrate": true,
	"goose":          true,
	"dbmate":         true,
	"flyway":         true,
	"liquibase":      true,
}

// validateMigrationFormat accepts an empty format (declarative mode) or one of
// the Atlas-supported migration directory formats.
func validateMigrationFormat(format string) error {
	if format == "" || migrationFormats[format] {
		return nil
	}
	return fmt.Errorf("unknown migration format %q", format)
}
