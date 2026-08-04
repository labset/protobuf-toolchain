package helpers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToSnake(t *testing.T) {
	tests := map[string]struct {
		in   string
		want string
	}{
		"empty":               {in: "", want: ""},
		"single word":         {in: "User", want: "user"},
		"already lower":       {in: "user", want: "user"},
		"two words":           {in: "OrderItem", want: "order_item"},
		"three words":         {in: "ProjectManagementService", want: "project_management_service"},
		"leading acronym":     {in: "APIKey", want: "api_key"},
		"acronym before word": {in: "HTTPServer", want: "http_server"},
		"trailing acronym":    {in: "UserID", want: "user_id"},
		"acronym only":        {in: "ID", want: "id"},
		"digit boundary":      {in: "S3Bucket", want: "s3_bucket"},
		"digit after word":    {in: "Oauth2Token", want: "oauth2_token"},
		"single upper":        {in: "A", want: "a"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.want, ToSnake(tt.in))
		})
	}
}
