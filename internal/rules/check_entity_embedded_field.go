package rules

import (
	"context"

	"buf.build/go/bufplugin/check"
	"buf.build/go/bufplugin/check/checkutil"
	pluginV1 "github.com/labset/protobuf-toolchain/api/labset/plugin/v1"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const (
	entityEmbeddedFieldRuleID = "LABSET_ENTITY_EMBEDDED_FIELD"
	entityMessageFullName     = "labset.plugin.v1.Entity"
)

var entityEmbeddedFieldRuleSpec = &check.RuleSpec{
	ID:      entityEmbeddedFieldRuleID,
	Default: true,
	Purpose: "Checks that a ROLE_ENTITY message embeds labset.plugin.v1.Entity at field number 1.",
	Type:    check.RuleTypeLint,
	Handler: checkutil.NewMessageRuleHandler(
		checkEntityEmbeddedField,
		checkutil.WithoutImports(),
	),
}

func checkEntityEmbeddedField(
	_ context.Context,
	responseWriter check.ResponseWriter,
	_ check.Request,
	messageDescriptor protoreflect.MessageDescriptor,
) error {
	annotation := messageAnnotation(messageDescriptor)
	if annotation == nil || annotation.GetRole() != pluginV1.Role_ROLE_ENTITY {
		return nil
	}

	field := messageDescriptor.Fields().ByNumber(1)
	if field == nil {
		responseWriter.AddAnnotation(
			check.WithMessagef(
				"Entity message %q must embed %s at field number 1, but has no field number 1.",
				messageDescriptor.Name(),
				entityMessageFullName,
			),
			check.WithDescriptor(messageDescriptor),
		)
		return nil
	}

	embedded := field.Message()
	if embedded == nil || field.IsList() || field.IsMap() ||
		string(embedded.FullName()) != entityMessageFullName {
		responseWriter.AddAnnotation(
			check.WithMessagef(
				"Entity message %q must embed a singular %s at field number 1, but field %q is %s.",
				messageDescriptor.Name(),
				entityMessageFullName,
				field.Name(),
				describeFieldType(field),
			),
			check.WithDescriptor(field),
		)
	}
	return nil
}

func describeFieldType(field protoreflect.FieldDescriptor) string {
	kind := field.Kind().String()
	if message := field.Message(); message != nil {
		kind = string(message.FullName())
	}
	switch {
	case field.IsMap():
		return "a map"
	case field.IsList():
		return "a repeated " + kind
	default:
		return "a " + kind
	}
}
