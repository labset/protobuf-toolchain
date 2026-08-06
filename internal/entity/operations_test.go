package entity

import (
	"testing"

	pluginV1 "github.com/labset/protobuf-toolchain/api/labset/plugin/v1"
	"github.com/stretchr/testify/assert"
)

func TestDistinctOperations(t *testing.T) {
	create := pluginV1.Operation_OPERATION_CREATE
	read := pluginV1.Operation_OPERATION_READ
	update := pluginV1.Operation_OPERATION_UPDATE
	unspecified := pluginV1.Operation_OPERATION_UNSPECIFIED

	tests := map[string]struct {
		ops  []pluginV1.Operation
		want []pluginV1.Operation
	}{
		"nil":   {ops: nil, want: nil},
		"empty": {ops: []pluginV1.Operation{}, want: nil},
		"drops unspecified": {
			ops:  []pluginV1.Operation{unspecified, create},
			want: []pluginV1.Operation{create},
		},
		"dedups preserving first order": {
			ops:  []pluginV1.Operation{read, create, read, create},
			want: []pluginV1.Operation{read, create},
		},
		"keeps distinct in given order": {
			ops:  []pluginV1.Operation{create, read, update},
			want: []pluginV1.Operation{create, read, update},
		},
		"only unspecified yields none": {
			ops:  []pluginV1.Operation{unspecified, unspecified},
			want: nil,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := DistinctOperations(&pluginV1.MessageOptions{Operations: tt.ops})
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestOperationName(t *testing.T) {
	tests := map[string]struct {
		op   pluginV1.Operation
		want string
	}{
		"create":      {op: pluginV1.Operation_OPERATION_CREATE, want: "CREATE"},
		"read":        {op: pluginV1.Operation_OPERATION_READ, want: "READ"},
		"update":      {op: pluginV1.Operation_OPERATION_UPDATE, want: "UPDATE"},
		"delete":      {op: pluginV1.Operation_OPERATION_DELETE, want: "DELETE"},
		"list":        {op: pluginV1.Operation_OPERATION_LIST, want: "LIST"},
		"unspecified": {op: pluginV1.Operation_OPERATION_UNSPECIFIED, want: "UNSPECIFIED"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.want, OperationName(tt.op))
		})
	}
}
