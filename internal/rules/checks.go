package rules

import (
	"buf.build/go/bufplugin/check"
)

var All = []*check.RuleSpec{
	entityAnnotationRootOnlyRuleSpec,
	entityEmbeddedFieldRuleSpec,
}
