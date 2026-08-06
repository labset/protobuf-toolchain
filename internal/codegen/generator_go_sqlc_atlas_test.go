package codegen

import (
	"path"
	"strings"
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
	plugin, err := generateSqlc(t, "", "",
		[]string{"projectmanagement/v1/project_management.proto"},
		sqlcProjectManagementFileDescriptor(),
	)
	require.NoError(t, err)

	resp := plugin.Response()
	require.Empty(t, resp.GetError())

	// Two entities in one package share a directory: one sql/schema.sql covers
	// both, each gets its own sql/queries/<table>.sql, and the config files sit
	// at the directory root.
	wantNames := []string{
		"projectmanagement/v1/sql/schema.sql",
		"projectmanagement/v1/sql/queries/project.sql",
		"projectmanagement/v1/sql/queries/task.sql",
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
	plugin, err := generateSqlc(t, "goose", "",
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

// TestGoSqlcAtlasGeneratorSchema verifies tables are qualified with a Postgres
// schema: derived from the proto package by default, or the explicit override.
func TestGoSqlcAtlasGeneratorSchema(t *testing.T) {
	tests := map[string]struct {
		schema string
		want   string
	}{
		"derived from package": {schema: "", want: "projectmanagement_v1"},
		"explicit override":    {schema: "app", want: "app"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			plugin, err := generateSqlc(t, "", tt.schema,
				[]string{"projectmanagement/v1/project_management.proto"},
				sqlcProjectManagementFileDescriptor(),
			)
			require.NoError(t, err)

			byName := make(map[string]string)
			for _, f := range plugin.Response().GetFile() {
				byName[path.Base(f.GetName())] = f.GetContent()
			}

			assert.Contains(t, byName["schema.sql"], "CREATE SCHEMA IF NOT EXISTS "+tt.want+";")
			assert.Contains(t, byName["schema.sql"], "CREATE TABLE "+tt.want+".project (")
			assert.Contains(t, byName["project.sql"], "INSERT INTO "+tt.want+".project (")
			assert.Contains(t, byName["atlas.hcl"], "search_path="+tt.want)
		})
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

			plugin, err := generateSqlc(t, "", "", []string{"skip/v1/thing.proto"}, file)
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

	plugin, err := generateSqlc(t, "", "", []string{"catalog/v1/catalog.proto"}, file)
	require.NoError(t, err)

	byName := make(map[string]string)
	for _, f := range plugin.Response().GetFile() {
		byName[path.Base(f.GetName())] = f.GetContent()
	}

	require.Contains(t, byName, "schema.sql")
	assert.Contains(t, byName["schema.sql"], "CREATE TABLE catalog_v1.tag")
	require.Contains(t, byName, "tag.sql")
	assert.NotContains(t, byName["tag.sql"], "-- name:")
}

// TestGoSqlcAtlasUnknownMigrationFormat verifies the mode rejects an unsupported
// migration format at dispatch time rather than emitting a broken atlas.hcl.
func TestGoSqlcAtlasUnknownMigrationFormat(t *testing.T) {
	_, err := GeneratorForMode("mode=go-sqlc-atlas,migration=bogus")
	require.ErrorContains(t, err, "unknown migration format")
}

// TestGoSqlcAtlasGeneratorQueriesAvoidWildcards verifies generated queries
// satisfy the SonarCloud checks: no SELECT */RETURNING * wildcard and an
// explicit ASC on the LIST ordering.
func TestGoSqlcAtlasGeneratorQueriesAvoidWildcards(t *testing.T) {
	plugin, err := generateSqlc(t, "", "",
		[]string{"projectmanagement/v1/project_management.proto"},
		sqlcProjectManagementFileDescriptor(),
	)
	require.NoError(t, err)

	for _, f := range plugin.Response().GetFile() {
		if path.Ext(f.GetName()) != ".sql" {
			continue
		}
		query := f.GetContent()
		assert.NotContains(t, query, "SELECT *", "%s uses SELECT *", f.GetName())
		assert.NotContains(t, query, "RETURNING *", "%s uses RETURNING *", f.GetName())
		if strings.Contains(query, "ORDER BY") {
			assert.Contains(t, query, "ORDER BY created_at ASC", "%s omits ASC", f.GetName())
		}
	}
}

// TestGoSqlcAtlasGeneratorRealOneofNullable verifies a member of a real (non
// -synthetic) oneof — which has presence but no optional keyword — maps to a
// nullable column, since only one arm is ever set.
func TestGoSqlcAtlasGeneratorRealOneofNullable(t *testing.T) {
	msgOpts := &descriptorpb.MessageOptions{}
	proto.SetExtension(msgOpts, pluginV1.E_Message, &pluginV1.MessageOptions{
		Role:       pluginV1.Role_ROLE_ENTITY,
		Operations: []pluginV1.Operation{pluginV1.Operation_OPERATION_CREATE},
	})
	contact := &descriptorpb.DescriptorProto{
		Name: proto.String("Contact"),
		Field: []*descriptorpb.FieldDescriptorProto{
			embeddedEntityField(1),
			oneofStringField("email", 2, 0),
			oneofStringField("phone", 3, 0),
		},
		OneofDecl: []*descriptorpb.OneofDescriptorProto{{Name: proto.String("reach")}},
		Options:   msgOpts,
	}

	file := &descriptorpb.FileDescriptorProto{
		Name:       proto.String("directory/v1/directory.proto"),
		Package:    proto.String("directory.v1"),
		Syntax:     proto.String("proto3"),
		Dependency: []string{"labset/plugin/v1/entity.proto", "labset/plugin/v1/options.proto"},
		Options: &descriptorpb.FileOptions{
			GoPackage: proto.String(
				"github.com/labset/protobuf-toolchain/test/directory/v1;directoryv1",
			),
		},
		MessageType: []*descriptorpb.DescriptorProto{contact},
	}

	plugin, err := generateSqlc(t, "", "", []string{"directory/v1/directory.proto"}, file)
	require.NoError(t, err)

	schema := generatedFile(t, plugin, "schema.sql")
	assert.Contains(t, schema, "  email text,")
	assert.Contains(t, schema, "  phone text,")
	assert.NotContains(t, schema, "email text NOT NULL")
}

// TestGoSqlcAtlasGeneratorNoPackageSchemaFallback verifies a package-less proto
// falls back to the "public" schema so the emitted SQL stays valid.
func TestGoSqlcAtlasGeneratorNoPackageSchemaFallback(t *testing.T) {
	file := &descriptorpb.FileDescriptorProto{
		Name:       proto.String("nopkg/thing.proto"),
		Syntax:     proto.String("proto3"),
		Dependency: []string{"labset/plugin/v1/entity.proto", "labset/plugin/v1/options.proto"},
		Options: &descriptorpb.FileOptions{
			GoPackage: proto.String("github.com/labset/protobuf-toolchain/test/nopkg;nopkg"),
		},
		MessageType: []*descriptorpb.DescriptorProto{
			sqlcEntityMessage("Thing", []*descriptorpb.FieldDescriptorProto{
				embeddedEntityField(1),
				stringField("label", 2),
			}, pluginV1.Operation_OPERATION_CREATE),
		},
	}

	plugin, err := generateSqlc(t, "", "", []string{"nopkg/thing.proto"}, file)
	require.NoError(t, err)

	schema := generatedFile(t, plugin, "schema.sql")
	assert.Contains(t, schema, "CREATE SCHEMA IF NOT EXISTS public;")
	assert.Contains(t, schema, "CREATE TABLE public.thing (")
}

// TestGoSqlcAtlasGeneratorMixedPackagesInDir verifies two proto packages sharing
// one output directory is an error rather than a silently mis-qualified schema.
func TestGoSqlcAtlasGeneratorMixedPackagesInDir(t *testing.T) {
	mk := func(name, pkg, message string) *descriptorpb.FileDescriptorProto {
		return &descriptorpb.FileDescriptorProto{
			Name:       proto.String(name),
			Package:    proto.String(pkg),
			Syntax:     proto.String("proto3"),
			Dependency: []string{"labset/plugin/v1/entity.proto", "labset/plugin/v1/options.proto"},
			Options: &descriptorpb.FileOptions{
				GoPackage: proto.String(
					"github.com/labset/protobuf-toolchain/test/shared/v1;sharedv1",
				),
			},
			MessageType: []*descriptorpb.DescriptorProto{
				sqlcEntityMessage(message, []*descriptorpb.FieldDescriptorProto{
					embeddedEntityField(1),
				}, pluginV1.Operation_OPERATION_CREATE),
			},
		}
	}

	_, err := generateSqlc(t, "", "",
		[]string{"shared/v1/a.proto", "shared/v1/b.proto"},
		mk("shared/v1/a.proto", "shared.v1", "Alpha"),
		mk("shared/v1/b.proto", "other.v1", "Beta"),
	)
	require.ErrorContains(t, err, "two proto packages")
}

// generateSqlc runs the go-sqlc-atlas generator (with the given migration
// format and schema override) over the given files plus the well-known and
// labset descriptor dependencies.
func generateSqlc(
	t *testing.T,
	migration, schema string,
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

	return plugin, (&goSqlcAtlasGenerator{migration: migration, schema: schema}).Generate(plugin)
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

// oneofStringField builds a string field that is a member of the real oneof at
// the given declaration index (presence without the proto3 optional keyword).
func oneofStringField(name string, number, oneofIndex int32) *descriptorpb.FieldDescriptorProto {
	field := stringField(name, number)
	field.OneofIndex = proto.Int32(oneofIndex)
	return field
}

// generatedFile returns the content of the emitted file with the given base name.
func generatedFile(t *testing.T, plugin *protogen.Plugin, base string) string {
	t.Helper()

	for _, f := range plugin.Response().GetFile() {
		if path.Base(f.GetName()) == base {
			return f.GetContent()
		}
	}
	require.Failf(t, "missing generated file", "no file named %q", base)
	return ""
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
