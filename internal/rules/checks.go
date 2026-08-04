package rules

import (
	"buf.build/go/bufplugin/check"
	pluginV1 "github.com/labset/protobuf-toolchain/api/labset/plugin/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

var All = []*check.RuleSpec{
	entityAnnotationRootOnlyRuleSpec,
	entityEmbeddedFieldRuleSpec,
}

// messageAnnotation returns the labset MessageOptions set on the message, or nil
// when it carries no labset annotation.
func messageAnnotation(messageDescriptor protoreflect.MessageDescriptor) *pluginV1.MessageOptions {
	options, ok := messageDescriptor.Options().(*descriptorpb.MessageOptions)
	if !ok || options == nil {
		return nil
	}
	if !proto.HasExtension(options, pluginV1.E_Message) {
		return nil
	}
	annotation, ok := proto.GetExtension(options, pluginV1.E_Message).(*pluginV1.MessageOptions)
	if !ok {
		return nil
	}
	return annotation
}
