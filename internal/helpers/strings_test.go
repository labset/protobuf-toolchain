package helpers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToSnake(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "single word", in: "User", want: "user"},
		{name: "already lower", in: "user", want: "user"},
		{name: "two words", in: "OrderItem", want: "order_item"},
		{name: "three words", in: "ProjectManagementService", want: "project_management_service"},
		{name: "leading acronym", in: "APIKey", want: "api_key"},
		{name: "acronym before word", in: "HTTPServer", want: "http_server"},
		{name: "trailing acronym", in: "UserID", want: "user_id"},
		{name: "acronym only", in: "ID", want: "id"},
		{name: "digit boundary", in: "S3Bucket", want: "s3_bucket"},
		{name: "digit after word", in: "Oauth2Token", want: "oauth2_token"},
		{name: "single upper", in: "A", want: "a"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ToSnake(tt.in))
		})
	}
}
