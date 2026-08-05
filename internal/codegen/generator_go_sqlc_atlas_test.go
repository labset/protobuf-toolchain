package codegen

import (
	"path"
	"testing"

	pluginV1 "github.com/labset/protobuf-toolchain/api/labset/plugin/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/pluginpb"
)

func TestGoSqlcAtlasGenerator(t *testing.T) {
	plugin, err := generateSqlc(t, "",
		[]string{"projectmanagement/v1/project_management.proto"},
		sqlcProjectManagementFileDescriptor(),
	)
	require.NoError(t, err)

	resp := plugin.Response()
	require.Empty(t, resp.GetError())

	// Two entities in one package aggregate into a single per-directory set:
	// one schema.sql and query.sql covering both, plus the shared config files.
	wantNames := []string{
		"projectmanagement/v1/schema.sql",
		"projectmanagement/v1/query.sql",
		"projectmanagement/v1/sqlc.yaml",
		"projectmanagement/v1/atlas.hcl",
		"projectmanagement/v1/generate.go",
	}

	files := resp.GetFile()
	gotNames := make([]string, len(files))
	for i, file := range files {
		gotNames[i] = file.GetName()
	}
	require.Equal(t, wantNames, gotNames)

	for _, file := range files {
		assertGolden(
			t,
			path.Join("go_sqlc_atlas", "declarative", path.Base(file.GetName())),
			file.GetContent(),
		)
	}
}

// TestGoSqlcAtlasGeneratorVersioned covers the migration-format variant: the
// config files switch to a versioned migrations directory, but the schema and
// queries are unchanged, so only the config artifacts have their own goldens.
func TestGoSqlcAtlasGeneratorVersioned(t *testing.T) {
	plugin, err := generateSqlc(t, "goose",
		[]string{"projectmanagement/v1/project_management.proto"},
		sqlcProjectManagementFileDescriptor(),
	)
	require.NoError(t, err)
	require.Empty(t, plugin.Response().GetError())

	configs := map[string]bool{"sqlc.yaml": true, "atlas.hcl": true, "generate.go": true}
	for _, file := range plugin.Response().GetFile() {
		base := path.Base(file.GetName())
		if !configs[base] {
			continue
		}
		assertGolden(t, path.Join("go_sqlc_atlas", "versioned", base), file.GetContent())
	}
}

// TestGoSqlcAtlasGeneratorSkips verifies that only ROLE_ENTITY messages produce
// output: an unannotated message and a reference-role message generate nothing.
func TestGoSqlcAtlasGeneratorSkips(t *testing.T) {
	tests := map[string]struct {
		annotate bool
		role     pluginV1.Role
	}{
		"no labset annotation": {annotate: false},
		"reference role":       {annotate: true, role: pluginV1.Role_ROLE_REFERENCE},
		"unspecified role":     {annotate: true, role: pluginV1.Role_ROLE_UNSPECIFIED},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			message := &descriptorpb.DescriptorProto{
				Name:  proto.String("Thing"),
				Field: []*descriptorpb.FieldDescriptorProto{embeddedEntityField(1)},
			}
			if tt.annotate {
				opts := &descriptorpb.MessageOptions{}
				proto.SetExtension(opts, pluginV1.E_Message, &pluginV1.MessageOptions{
					Role:       tt.role,
					Operations: []pluginV1.Operation{pluginV1.Operation_OPERATION_CREATE},
				})
				message.Options = opts
			}

			file := &descriptorpb.FileDescriptorProto{
				Name:    proto.String("skip/v1/thing.proto"),
				Package: proto.String("skip.v1"),
				Syntax:  proto.String("proto3"),
				Dependency: []string{
					"labset/plugin/v1/entity.proto",
					"labset/plugin/v1/options.proto",
				},
				Options: &descriptorpb.FileOptions{
					GoPackage: proto.String(
						"github.com/labset/protobuf-toolchain/test/skip/v1;skipv1",
					),
				},
				MessageType: []*descriptorpb.DescriptorProto{message},
			}

			plugin, err := generateSqlc(t, "", []string{"skip/v1/thing.proto"}, file)
			require.NoError(t, err)
			require.Empty(t, plugin.Response().GetFile())
		})
	}
}

// TestGoSqlcAtlasGeneratorEntityWithoutOperations verifies a ROLE_ENTITY with no
// operations still gets a table (and config), but query.sql carries no queries.
func TestGoSqlcAtlasGeneratorEntityWithoutOperations(t *testing.T) {
	file := &descriptorpb.FileDescriptorProto{
		Name:       proto.String("catalog/v1/catalog.proto"),
		Package:    proto.String("catalog.v1"),
		Syntax:     proto.String("proto3"),
		Dependency: []string{"labset/plugin/v1/entity.proto", "labset/plugin/v1/options.proto"},
		Options: &descriptorpb.FileOptions{
			GoPackage: proto.String(
				"github.com/labset/protobuf-toolchain/test/catalog/v1;catalogv1",
			),
		},
		MessageType: []*descriptorpb.DescriptorProto{
			sqlcEntityMessage("Tag", []*descriptorpb.FieldDescriptorProto{
				embeddedEntityField(1),
				stringField("label", 2),
			}),
		},
	}

	plugin, err := generateSqlc(t, "", []string{"catalog/v1/catalog.proto"}, file)
	require.NoError(t, err)

	byName := make(map[string]string)
	for _, f := range plugin.Response().GetFile() {
		byName[path.Base(f.GetName())] = f.GetContent()
	}

	require.Contains(t, byName, "schema.sql")
	assert.Contains(t, byName["schema.sql"], "CREATE TABLE tag")
	assert.NotContains(t, byName["query.sql"], "-- name:")
}

// TestGoSqlcAtlasUnknownMigrationFormat verifies the mode rejects an unsupported
// migration format at dispatch time rather than emitting a broken atlas.hcl.
func TestGoSqlcAtlasUnknownMigrationFormat(t *testing.T) {
	_, err := GeneratorForMode("mode=go-sqlc-atlas,migration=bogus")
	require.ErrorContains(t, err, "unknown migration format")
}

// generateSqlc runs the go-sqlc-atlas generator (with the given migration
// format) over the given files plus the well-known and labset descriptor
// dependencies.
func generateSqlc(
	t *testing.T,
	migration string,
	toGenerate []string,
	files ...*descriptorpb.FileDescriptorProto,
) (*protogen.Plugin, error) {
	t.Helper()

	protoFiles := []*descriptorpb.FileDescriptorProto{
		protodesc.ToFileDescriptorProto(descriptorpb.File_google_protobuf_descriptor_proto),
		protodesc.ToFileDescriptorProto(timestamppb.File_google_protobuf_timestamp_proto),
		protodesc.ToFileDescriptorProto(pluginV1.File_labset_plugin_v1_enums_proto),
		protodesc.ToFileDescriptorProto(pluginV1.File_labset_plugin_v1_options_proto),
		protodesc.ToFileDescriptorProto(pluginV1.File_labset_plugin_v1_entity_proto),
	}
	protoFiles = append(protoFiles, files...)

	plugin, err := protogen.Options{}.New(&pluginpb.CodeGeneratorRequest{
		FileToGenerate: toGenerate,
		ProtoFile:      protoFiles,
	})
	require.NoError(t, err)

	return plugin, (&goSqlcAtlasGenerator{migration: migration}).Generate(plugin)
}

// sqlcProjectManagementFileDescriptor builds a project-management proto with a
// Project entity (full CRUD) and a Task entity (create/read/list), each
// embedding labset.plugin.v1.Entity at field 1.
func sqlcProjectManagementFileDescriptor() *descriptorpb.FileDescriptorProto {
	return &descriptorpb.FileDescriptorProto{
		Name:       proto.String("projectmanagement/v1/project_management.proto"),
		Package:    proto.String("projectmanagement.v1"),
		Syntax:     proto.String("proto3"),
		Dependency: []string{"labset/plugin/v1/entity.proto", "labset/plugin/v1/options.proto"},
		Options: &descriptorpb.FileOptions{
			GoPackage: proto.String(
				"github.com/labset/protobuf-toolchain/test/projectmanagement/v1;projectmanagementv1",
			),
		},
		MessageType: []*descriptorpb.DescriptorProto{
			sqlcEntityMessage("Project",
				[]*descriptorpb.FieldDescriptorProto{
					embeddedEntityField(1),
					stringField("name", 2),
					stringField("description", 3),
				},
				pluginV1.Operation_OPERATION_CREATE,
				pluginV1.Operation_OPERATION_READ,
				pluginV1.Operation_OPERATION_UPDATE,
				pluginV1.Operation_OPERATION_DELETE,
				pluginV1.Operation_OPERATION_LIST,
			),
			sqlcEntityMessage("Task",
				[]*descriptorpb.FieldDescriptorProto{
					embeddedEntityField(1),
					stringField("project_id", 2),
					stringField("title", 3),
					optionalInt64Field("priority", 4),
				},
				pluginV1.Operation_OPERATION_CREATE,
				pluginV1.Operation_OPERATION_READ,
				pluginV1.Operation_OPERATION_LIST,
			),
		},
	}
}

// sqlcEntityMessage builds a ROLE_ENTITY message carrying the given operations.
func sqlcEntityMessage(
	name string,
	fields []*descriptorpb.FieldDescriptorProto,
	ops ...pluginV1.Operation,
) *descriptorpb.DescriptorProto {
	msgOpts := &descriptorpb.MessageOptions{}
	proto.SetExtension(msgOpts, pluginV1.E_Message, &pluginV1.MessageOptions{
		Role:       pluginV1.Role_ROLE_ENTITY,
		Operations: ops,
	})

	message := &descriptorpb.DescriptorProto{
		Name:    proto.String(name),
		Field:   fields,
		Options: msgOpts,
	}

	// proto3 optional fields must each sit in their own synthetic oneof,
	// declared after any real oneofs; protodesc rejects the descriptor otherwise.
	for _, field := range fields {
		if field.GetProto3Optional() {
			field.OneofIndex = proto.Int32(int32(len(message.GetOneofDecl())))
			message.OneofDecl = append(message.OneofDecl,
				&descriptorpb.OneofDescriptorProto{Name: proto.String("_" + field.GetName())},
			)
		}
	}

	return message
}

// embeddedEntityField builds the labset.plugin.v1.Entity base field.
func embeddedEntityField(number int32) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name:     proto.String("entity"),
		Number:   proto.Int32(number),
		Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
		TypeName: proto.String(".labset.plugin.v1.Entity"),
	}
}

// optionalInt64Field builds a proto3 optional int64 field (a nullable column).
func optionalInt64Field(name string, number int32) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name:           proto.String(name),
		Number:         proto.Int32(number),
		Label:          descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		Type:           descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum(),
		Proto3Optional: proto.Bool(true),
	}
}
