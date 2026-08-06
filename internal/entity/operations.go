package entity

import (
	"strings"

	pluginV1 "github.com/labset/protobuf-toolchain/api/labset/plugin/v1"
)

// DistinctOperations returns the annotation's operations with
// OPERATION_UNSPECIFIED and duplicates removed, preserving first-seen order.
func DistinctOperations(opts *pluginV1.MessageOptions) []pluginV1.Operation {
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

// OperationName is the bare token for an operation: OPERATION_CREATE -> "CREATE".
func OperationName(op pluginV1.Operation) string {
	return strings.TrimPrefix(op.String(), "OPERATION_")
}
