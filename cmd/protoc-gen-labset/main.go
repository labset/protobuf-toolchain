package main

import (
	"github.com/labset/protobuf-toolchain/internal/codegen"
	"google.golang.org/protobuf/compiler/protogen"
)

func main() {
	protogen.Options{}.Run(func(plugin *protogen.Plugin) error {
		generator, err := codegen.GeneratorForMode(plugin.Request.GetParameter())
		if err != nil {
			return err
		}

		return generator.Generate(plugin)
	})
}
