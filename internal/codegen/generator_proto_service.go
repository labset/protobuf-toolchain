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
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

//go:embed templates/proto-service/*.tmpl
var protoServiceTemplateFS embed.FS

// protoServiceGenerator emits, for every message annotated as ROLE_ENTITY with a
// non-empty operation set, one rpc_<operation>_<entity>.proto per operation
// (carrying that operation's request/response payloads) plus a
// service_<entity>.proto tying the RPCs together. The per-operation shape lives
// entirely in the templates; this generator only discovers entities and drives
// template execution.
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

		for _, message := range file.Messages {
			if err = g.generateForMessage(plugin, tmpl, file, message, seenPaths); err != nil {
				return err
			}
			if err = ensureNoNestedEntities(message); err != nil {
				return err
			}
		}
	}

	return nil
}

// ensureNoNestedEntities rejects entity annotations on nested messages. Only
// top-level messages model first-class resources; a nested message is scoped to
// its parent and cannot be referenced by an unqualified name from a generated
// file, so annotating one is a mistake worth surfacing rather than silently
// skipping or emitting an invalid type reference.
func ensureNoNestedEntities(message *protogen.Message) error {
	for _, nested := range message.Messages {
		if _, ok := entityOperations(nested); ok {
			return fmt.Errorf(
				"proto-service: nested message %s is annotated as an entity; only top-level messages are supported",
				nested.Desc.FullName(),
			)
		}
		if err := ensureNoNestedEntities(nested); err != nil {
			return err
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
	seen := make(map[pluginV1.Operation]bool)
	for _, op := range operations {
		if op == pluginV1.Operation_OPERATION_UNSPECIFIED || seen[op] {
			continue
		}
		seen[op] = true

		name := strings.TrimPrefix(op.String(), "OPERATION_")
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

	if len(service.Operations) == 0 {
		return nil
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

// entityData is the naming information shared by both templates. It carries no
// operation-specific logic — the templates decide what each operation renders.
// List operations use a deterministic "<Entity>Collection" naming (never a
// fragile pluralization) so any entity name works.
type entityData struct {
	Package     string
	Source      string
	Entity      string // Project
	EntityField string // project
}

// rpcModel is the input for rpc.proto.tmpl: one entity plus one operation token.
type rpcModel struct {
	entityData
	Operation string // CREATE | READ | UPDATE | DELETE | LIST
}

// serviceModel is the input for service.proto.tmpl: one entity plus the ordered
// list of operations and the payload files to import.
type serviceModel struct {
	entityData
	Imports    []string
	Operations []string
}

// entityOperations returns the operation set when the message is a ROLE_ENTITY
// carrying at least one operation, else reports false.
func entityOperations(message *protogen.Message) ([]pluginV1.Operation, bool) {
	opts, ok := message.Desc.Options().(*descriptorpb.MessageOptions)
	if !ok || opts == nil {
		return nil, false
	}
	if !proto.HasExtension(opts, pluginV1.E_Message) {
		return nil, false
	}

	msgOpts, ok := proto.GetExtension(opts, pluginV1.E_Message).(*pluginV1.MessageOptions)
	if !ok || msgOpts == nil {
		return nil, false
	}
	if msgOpts.GetRole() != pluginV1.Role_ROLE_ENTITY || len(msgOpts.GetOperations()) == 0 {
		return nil, false
	}

	return msgOpts.GetOperations(), true
}
