package rules

import "testing"

func TestEntityEmbeddedField(t *testing.T) {
	t.Parallel()

	const checkName = "entity_embedded_field"

	tests := map[string]struct {
		caseName string
		want     []annotation
	}{
		"embeds entity at field one": {
			caseName: "valid",
			want:     nil,
		},
		"missing field one is flagged": {
			caseName: "missing",
			want:     []annotation{{entityEmbeddedFieldRuleID, "missing.proto"}},
		},
		"wrong field one type is flagged": {
			caseName: "wrong_type",
			want:     []annotation{{entityEmbeddedFieldRuleID, "wrong_type.proto"}},
		},
		"repeated entity at field one is flagged": {
			caseName: "repeated",
			want:     []annotation{{entityEmbeddedFieldRuleID, "repeated.proto"}},
		},
		"non-entity messages are ignored": {
			caseName: "not_entity",
			want:     nil,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assertRule(t, entityEmbeddedFieldRuleID, checkName, tt.caseName, tt.want)
		})
	}
}
