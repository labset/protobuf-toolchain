package codegen

import "embed"

//go:embed templates/proto-service/*.tmpl
var protoServiceTemplateFS embed.FS
