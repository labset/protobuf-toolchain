package entity

import (
	pluginV1 "github.com/labset/protobuf-toolchain/api/labset/plugin/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// BaseFullName is the message a ROLE_ENTITY embeds at field 1.
const BaseFullName = "labset.plugin.v1.Entity"

// MessageAnnotation returns the labset.plugin.v1.MessageOptions set on a
// message, or nil when it carries none.
func MessageAnnotation(md protoreflect.MessageDescriptor) *pluginV1.MessageOptions {
	opts, ok := md.Options().(*descriptorpb.MessageOptions)
	if !ok || opts == nil {
		return nil
	}
	if !proto.HasExtension(opts, pluginV1.E_Message) {
		return nil
	}

	annotation, ok := proto.GetExtension(opts, pluginV1.E_Message).(*pluginV1.MessageOptions)
	if !ok {
		return nil
	}
	return annotation
}
