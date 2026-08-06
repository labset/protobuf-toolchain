package rules

import (
	"context"

	"buf.build/go/bufplugin/check"
	"buf.build/go/bufplugin/check/checkutil"
	pluginV1 "github.com/labset/protobuf-toolchain/api/labset/plugin/v1"
	"github.com/labset/protobuf-toolchain/internal/entity"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const entityAnnotationRootOnlyRuleID = "LABSET_ENTITY_ANNOTATION_ROOT_ONLY"

var entityAnnotationRootOnlyRuleSpec = &check.RuleSpec{
	ID:      entityAnnotationRootOnlyRuleID,
	Default: true,
	Purpose: "Checks that labset entity role/operations annotations are only applied to top-level messages.",
	Type:    check.RuleTypeLint,
	Handler: checkutil.NewMessageRuleHandler(
		checkEntityAnnotationRootOnly,
		checkutil.WithoutImports(),
	),
}

// checkEntityAnnotationRootOnly flags a nested annotated message. Only top-level
// messages model first-class resources; the codegen silently skips a nested one,
// so this surfaces the misplacement instead.
func checkEntityAnnotationRootOnly(
	_ context.Context,
	responseWriter check.ResponseWriter,
	_ check.Request,
	messageDescriptor protoreflect.MessageDescriptor,
) error {
	annotation := entity.MessageAnnotation(messageDescriptor)
	if annotation == nil {
		return nil
	}
	// An empty annotation asserts no entity semantics, so there is nothing to
	// place correctly.
	if annotation.GetRole() == pluginV1.Role_ROLE_UNSPECIFIED &&
		len(annotation.GetOperations()) == 0 {
		return nil
	}
	if _, ok := messageDescriptor.Parent().(protoreflect.FileDescriptor); ok {
		return nil
	}

	responseWriter.AddAnnotation(
		check.WithMessagef(
			"Entity annotation must be on a top-level message; %q is nested in %q.",
			messageDescriptor.Name(),
			messageDescriptor.Parent().FullName(),
		),
		check.WithDescriptor(messageDescriptor),
	)
	return nil
}
