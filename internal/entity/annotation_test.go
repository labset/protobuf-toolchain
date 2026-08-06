package entity

import (
	"testing"

	pluginV1 "github.com/labset/protobuf-toolchain/api/labset/plugin/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestMessageAnnotation(t *testing.T) {
	annotated := &descriptorpb.MessageOptions{}
	proto.SetExtension(annotated, pluginV1.E_Message, &pluginV1.MessageOptions{
		Role: pluginV1.Role_ROLE_ENTITY,
	})

	tests := map[string]struct {
		options  *descriptorpb.MessageOptions
		wantNil  bool
		wantRole pluginV1.Role
	}{
		"no options":                 {options: nil, wantNil: true},
		"options without labset ext": {options: &descriptorpb.MessageOptions{}, wantNil: true},
		"annotated":                  {options: annotated, wantRole: pluginV1.Role_ROLE_ENTITY},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			md := messageDescriptor(t, &descriptorpb.DescriptorProto{
				Name:    proto.String("Thing"),
				Options: tt.options,
			})

			got := MessageAnnotation(md)
			if tt.wantNil {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.Equal(t, tt.wantRole, got.GetRole())
		})
	}
}

// messageDescriptor compiles a one-message proto (with the labset options
// dependency available so the annotation resolves) and returns its descriptor.
func messageDescriptor(
	t *testing.T,
	message *descriptorpb.DescriptorProto,
) protoreflect.MessageDescriptor {
	t.Helper()

	file := &descriptorpb.FileDescriptorProto{
		Name:        proto.String("entitytest/v1/test.proto"),
		Package:     proto.String("entitytest.v1"),
		Syntax:      proto.String("proto3"),
		Dependency:  []string{"labset/plugin/v1/options.proto"},
		MessageType: []*descriptorpb.DescriptorProto{message},
	}

	files, err := protodesc.NewFiles(&descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{
			protodesc.ToFileDescriptorProto(descriptorpb.File_google_protobuf_descriptor_proto),
			protodesc.ToFileDescriptorProto(pluginV1.File_labset_plugin_v1_enums_proto),
			protodesc.ToFileDescriptorProto(pluginV1.File_labset_plugin_v1_options_proto),
			file,
		},
	})
	require.NoError(t, err)

	fd, err := files.FindFileByPath("entitytest/v1/test.proto")
	require.NoError(t, err)
	return fd.Messages().Get(0)
}
