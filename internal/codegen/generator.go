package codegen

import (
	"google.golang.org/protobuf/compiler/protogen"
)

type Generator interface {
	Generate(plugin *protogen.Plugin) error
}
