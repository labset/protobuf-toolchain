package codegen

import (
	"fmt"

	"google.golang.org/protobuf/compiler/protogen"
)

type echoGenerator struct{}

func (g *echoGenerator) Generate(plugin *protogen.Plugin) error {
	for _, file := range plugin.Files {
		if !file.Generate {
			continue
		}

		fmt.Printf("Generating %s...\n", file.GeneratedFilenamePrefix)
	}

	return nil
}
