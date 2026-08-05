package codegen

import (
	"embed"
	"fmt"
	"path"
	"strings"
	"text/template"

	pluginV1 "github.com/labset/protobuf-toolchain/api/labset/plugin/v1"
	"github.com/labset/protobuf-toolchain/internal/helpers"
	"google.golang.org/protobuf/compiler/protogen"
)

//go:embed templates/proto-service/*.tmpl
var protoServiceTemplateFS embed.FS

// protoServiceGenerator emits, for every message annotated as ROLE_ENTITY with a
// non-empty operation set, one rpc_<operation>_<entity>.proto per operation
// (carrying that operation's request/response payloads) plus a
// service_<entity>.proto tying the RPCs together.
type protoServiceGenerator struct{}

func (g *protoServiceGenerator) Generate(plugin *protogen.Plugin) error {
	tmpl, err := template.ParseFS(protoServiceTemplateFS, "templates/proto-service/*.tmpl")
	if err != nil {
		return err
	}

	seenPaths := make(map[string]string)
	for _, file := range plugin.Files {
		if !file.Generate {
			continue
		}

		// Only top-level messages model first-class resources. A nested message
		// is scoped to its parent and cannot be referenced by an unqualified name
		// from a generated file, so an entity annotation there is skipped —
		// enforcing that placement is a lint concern, not a codegen failure.
		for _, message := range file.Messages {
			if err = g.generateForMessage(plugin, tmpl, file, message, seenPaths); err != nil {
				return err
			}
		}
	}

	return nil
}

func (g *protoServiceGenerator) generateForMessage(
	plugin *protogen.Plugin,
	tmpl *template.Template,
	file *protogen.File,
	message *protogen.Message,
	seenPaths map[string]string,
) error {
	operations, ok := entityOperations(message)
	if !ok {
		return nil
	}

	entity := string(message.Desc.Name())
	data := entityData{
		Package:     string(file.Desc.Package()),
		Source:      file.Desc.Path(),
		Entity:      entity,
		EntityField: helpers.ToSnake(entity),
	}
	dir := path.Dir(data.Source)

	service := serviceModel{entityData: data}
	for _, op := range operations {
		name := operationName(op)
		filename := path.Join(dir, "rpc_"+strings.ToLower(name)+"_"+data.EntityField+".proto")

		out, err := newGeneratedFile(plugin, seenPaths, filename, entity, file.GoImportPath)
		if err != nil {
			return err
		}
		if err = tmpl.ExecuteTemplate(
			out,
			"rpc.proto.tmpl",
			rpcModel{entityData: data, Operation: name},
		); err != nil {
			return err
		}

		service.Operations = append(service.Operations, name)
		service.Imports = append(service.Imports, filename)
	}

	servicePath := path.Join(dir, "service_"+data.EntityField+".proto")
	out, err := newGeneratedFile(plugin, seenPaths, servicePath, entity, file.GoImportPath)
	if err != nil {
		return err
	}

	return tmpl.ExecuteTemplate(out, "service.proto.tmpl", service)
}

// newGeneratedFile registers filename and creates the output file, returning an
// error if a different entity already claimed the same path — distinct entity
// names can snake-case into the same file in a directory, which protoc rejects
// as a duplicate generated file.
func newGeneratedFile(
	plugin *protogen.Plugin,
	seenPaths map[string]string,
	filename, entity string,
	importPath protogen.GoImportPath,
) (*protogen.GeneratedFile, error) {
	if prev, ok := seenPaths[filename]; ok {
		return nil, fmt.Errorf(
			"proto-service: entities %q and %q both generate %q",
			prev, entity, filename,
		)
	}
	seenPaths[filename] = entity

	return plugin.NewGeneratedFile(filename, importPath), nil
}

// entityData holds the naming fields shared by both templates.
type entityData struct {
	Package     string
	Source      string
	Entity      string // Project
	EntityField string // project
}

// rpcModel is the input for rpc.proto.tmpl.
type rpcModel struct {
	entityData
	Operation string // CREATE | READ | UPDATE | DELETE | LIST
}

// serviceModel is the input for service.proto.tmpl.
type serviceModel struct {
	entityData
	Imports    []string
	Operations []string
}

// entityOperations returns the distinct, generatable operations for a message
// when it is a ROLE_ENTITY, else reports false. OPERATION_UNSPECIFIED and
// duplicates are dropped, so a message that requests no real operation is not
// generatable.
func entityOperations(message *protogen.Message) ([]pluginV1.Operation, bool) {
	opts := entityMessageOptions(message)
	if opts == nil || opts.GetRole() != pluginV1.Role_ROLE_ENTITY {
		return nil, false
	}

	operations := distinctOperations(opts)
	return operations, len(operations) > 0
}
