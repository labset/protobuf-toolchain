package codegen

import (
	"flag"
	"os"
	"path"
	"path/filepath"
	"testing"

	pluginV1 "github.com/labset/protobuf-toolchain/api/labset/plugin/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

var update = flag.Bool("update", false, "update golden files")

func TestProtoServiceGenerator(t *testing.T) {
	plugin, err := generate(t,
		[]string{"projectmanagement/v1/project_management.proto"},
		projectManagementFileDescriptor(t),
	)
	require.NoError(t, err)

	resp := plugin.Response()
	require.Empty(t, resp.GetError())

	// Project supports full CRUD; Task supports a create/read/list subset. Files
	// are emitted per entity, in operation order, with the service file last.
	wantNames := []string{
		"projectmanagement/v1/rpc_create_project.proto",
		"projectmanagement/v1/rpc_read_project.proto",
		"projectmanagement/v1/rpc_update_project.proto",
		"projectmanagement/v1/rpc_delete_project.proto",
		"projectmanagement/v1/rpc_list_project.proto",
		"projectmanagement/v1/service_project.proto",
		"projectmanagement/v1/rpc_create_task.proto",
		"projectmanagement/v1/rpc_read_task.proto",
		"projectmanagement/v1/rpc_list_task.proto",
		"projectmanagement/v1/service_task.proto",
	}

	files := resp.GetFile()
	gotNames := make([]string, len(files))
	for i, file := range files {
		gotNames[i] = file.GetName()
	}
	require.Equal(t, wantNames, gotNames)

	for _, file := range files {
		assertGolden(t, path.Join("proto_service", path.Base(file.GetName())), file.GetContent())
	}
}

// TestProtoServiceGeneratorNestedEntityRejected verifies an entity annotation on
// a nested message is reported as an error — nested messages are not first-class
// resources and cannot be referenced by an unqualified name from a generated
// file, so they must not be silently skipped or emitted as invalid output.
func TestProtoServiceGeneratorNestedEntityRejected(t *testing.T) {
	accountFile := &descriptorpb.FileDescriptorProto{
		Name:       proto.String("nested/v1/account.proto"),
		Package:    proto.String("nested.v1"),
		Syntax:     proto.String("proto3"),
		Dependency: []string{"labset/plugin/v1/options.proto"},
		Options: &descriptorpb.FileOptions{
			GoPackage: proto.String("github.com/labset/protobuf-toolchain/test/nested/v1;nestedv1"),
		},
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: proto.String("Account"),
			NestedType: []*descriptorpb.DescriptorProto{
				entityMessage("Profile",
					[]*descriptorpb.FieldDescriptorProto{stringField("id", 1)},
					pluginV1.Operation_OPERATION_CREATE,
				),
			},
		}},
	}

	_, err := generate(t, []string{"nested/v1/account.proto"}, accountFile)
	require.ErrorContains(t, err, "nested.v1.Account.Profile")
}

// TestProtoServiceGeneratorDuplicatePath verifies distinct entities that resolve
// to the same output path produce an error instead of duplicate output files.
func TestProtoServiceGeneratorDuplicatePath(t *testing.T) {
	// Proto names are case-sensitive, so OrderId and OrderID are two valid,
	// distinct messages in one package — but both snake-case to "order_id" and
	// so claim the same rpc_create_order_id.proto.
	orders := &descriptorpb.FileDescriptorProto{
		Name:       proto.String("orders/v1/orders.proto"),
		Package:    proto.String("orders.v1"),
		Syntax:     proto.String("proto3"),
		Dependency: []string{"labset/plugin/v1/options.proto"},
		Options: &descriptorpb.FileOptions{
			GoPackage: proto.String("github.com/labset/protobuf-toolchain/test/orders/v1;ordersv1"),
		},
		MessageType: []*descriptorpb.DescriptorProto{
			entityMessage("OrderId",
				[]*descriptorpb.FieldDescriptorProto{stringField("id", 1)},
				pluginV1.Operation_OPERATION_CREATE,
			),
			entityMessage("OrderID",
				[]*descriptorpb.FieldDescriptorProto{stringField("id", 1)},
				pluginV1.Operation_OPERATION_CREATE,
			),
		},
	}

	_, err := generate(t, []string{"orders/v1/orders.proto"}, orders)
	require.ErrorContains(t, err, "both generate")
}

// generate runs the proto-service generator over the given files (plus the
// well-known and labset descriptor dependencies) and returns the plugin so the
// caller can inspect its response.
func generate(
	t *testing.T,
	toGenerate []string,
	files ...*descriptorpb.FileDescriptorProto,
) (*protogen.Plugin, error) {
	t.Helper()

	protoFiles := []*descriptorpb.FileDescriptorProto{
		protodesc.ToFileDescriptorProto(descriptorpb.File_google_protobuf_descriptor_proto),
		protodesc.ToFileDescriptorProto(pluginV1.File_labset_plugin_v1_enums_proto),
		protodesc.ToFileDescriptorProto(pluginV1.File_labset_plugin_v1_options_proto),
	}
	protoFiles = append(protoFiles, files...)

	plugin, err := protogen.Options{}.New(&pluginpb.CodeGeneratorRequest{
		FileToGenerate: toGenerate,
		ProtoFile:      protoFiles,
	})
	require.NoError(t, err)

	return plugin, (&protoServiceGenerator{}).Generate(plugin)
}

// projectManagementFileDescriptor builds a project-management proto with a
// Project entity (full CRUD) and a Task entity (create/read/list).
func projectManagementFileDescriptor(t *testing.T) *descriptorpb.FileDescriptorProto {
	t.Helper()

	return &descriptorpb.FileDescriptorProto{
		Name:       proto.String("projectmanagement/v1/project_management.proto"),
		Package:    proto.String("projectmanagement.v1"),
		Syntax:     proto.String("proto3"),
		Dependency: []string{"labset/plugin/v1/options.proto"},
		Options: &descriptorpb.FileOptions{
			GoPackage: proto.String(
				"github.com/labset/protobuf-toolchain/test/projectmanagement/v1;projectmanagementv1",
			),
		},
		MessageType: []*descriptorpb.DescriptorProto{
			entityMessage("Project",
				[]*descriptorpb.FieldDescriptorProto{
					stringField("id", 1),
					stringField("name", 2),
					stringField("description", 3),
				},
				pluginV1.Operation_OPERATION_CREATE,
				pluginV1.Operation_OPERATION_READ,
				pluginV1.Operation_OPERATION_UPDATE,
				pluginV1.Operation_OPERATION_DELETE,
				pluginV1.Operation_OPERATION_LIST,
			),
			entityMessage("Task",
				[]*descriptorpb.FieldDescriptorProto{
					stringField("id", 1),
					stringField("project_id", 2),
					stringField("title", 3),
				},
				pluginV1.Operation_OPERATION_CREATE,
				pluginV1.Operation_OPERATION_READ,
				pluginV1.Operation_OPERATION_LIST,
			),
		},
	}
}

// entityMessage builds a message annotated as a ROLE_ENTITY carrying the given
// CRUD operations.
func entityMessage(
	name string,
	fields []*descriptorpb.FieldDescriptorProto,
	ops ...pluginV1.Operation,
) *descriptorpb.DescriptorProto {
	msgOpts := &descriptorpb.MessageOptions{}
	proto.SetExtension(msgOpts, pluginV1.E_Message, &pluginV1.MessageOptions{
		Role:       pluginV1.Role_ROLE_ENTITY,
		Operations: ops,
	})

	return &descriptorpb.DescriptorProto{
		Name:    proto.String(name),
		Field:   fields,
		Options: msgOpts,
	}
}

func stringField(name string, number int32) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name:   proto.String(name),
		Number: proto.Int32(number),
		Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
	}
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()

	goldenPath := filepath.Join("golden", filepath.FromSlash(name))
	if *update {
		require.NoError(t, os.MkdirAll(filepath.Dir(goldenPath), 0o755))
		require.NoError(t, os.WriteFile(goldenPath, []byte(got), 0o644))
		return
	}

	want, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "read golden %s (run with -update to create)", goldenPath)
	assert.Equal(t, string(want), got, "generated output does not match %s", goldenPath)
}
