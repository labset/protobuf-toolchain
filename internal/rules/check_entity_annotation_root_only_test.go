package rules

import "testing"

func TestEntityAnnotationRootOnly(t *testing.T) {
	t.Parallel()

	const checkName = "entity_annotation_root_only"

	tests := map[string]struct {
		caseName string
		want     []annotation
	}{
		"top-level annotation is allowed": {
			caseName: "top_level",
			want:     nil,
		},
		"nested annotation is flagged": {
			caseName: "nested",
			want:     []annotation{{entityAnnotationRootOnlyRuleID, "nested.proto"}},
		},
		"empty annotation on nested message is ignored": {
			caseName: "empty_nested",
			want:     nil,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assertRule(t, entityAnnotationRootOnlyRuleID, checkName, tt.caseName, tt.want)
		})
	}
}
