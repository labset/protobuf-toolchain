package codegen

import (
	"strings"

	pluginV1 "github.com/labset/protobuf-toolchain/api/labset/plugin/v1"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// entityBaseFullName is the message every ROLE_ENTITY embeds for its identity
// and lifecycle columns; it is never itself a column.
const entityBaseFullName = "labset.plugin.v1.Entity"

// entityMessageOptions returns the labset.plugin.v1.MessageOptions annotation on
// a message, or nil when it carries none. Shared by every generator that reads
// the entity annotation.
func entityMessageOptions(message *protogen.Message) *pluginV1.MessageOptions {
	opts, ok := message.Desc.Options().(*descriptorpb.MessageOptions)
	if !ok || opts == nil {
		return nil
	}
	if !proto.HasExtension(opts, pluginV1.E_Message) {
		return nil
	}

	msgOpts, ok := proto.GetExtension(opts, pluginV1.E_Message).(*pluginV1.MessageOptions)
	if !ok {
		return nil
	}
	return msgOpts
}

// distinctOperations returns the annotation's operations with OPERATION_UNSPECIFIED
// and duplicates removed, preserving first-seen order.
func distinctOperations(opts *pluginV1.MessageOptions) []pluginV1.Operation {
	seen := make(map[pluginV1.Operation]bool)
	var operations []pluginV1.Operation
	for _, op := range opts.GetOperations() {
		if op == pluginV1.Operation_OPERATION_UNSPECIFIED || seen[op] {
			continue
		}
		seen[op] = true
		operations = append(operations, op)
	}
	return operations
}

// operationName is the bare CRUD token for an operation (OPERATION_CREATE ->
// "CREATE"), the form both the proto-service and go-sqlc-atlas templates branch on.
func operationName(op pluginV1.Operation) string {
	return strings.TrimPrefix(op.String(), "OPERATION_")
}
