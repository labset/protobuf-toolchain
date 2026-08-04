package rules

import (
	"context"
	"path/filepath"
	"sort"
	"testing"

	"buf.build/go/bufplugin/check"
	"buf.build/go/bufplugin/check/checktest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// protosDir lets fixtures import the labset protos from the real schema module
// rather than from copies, so the options extension the rules decode always
// matches what ships.
const protosDir = "../../protos"

func TestSpec(t *testing.T) {
	t.Parallel()

	checktest.SpecTest(t, &check.Spec{Rules: All})
}

type annotation struct {
	ruleID string
	file   string
}

func assertRule(t *testing.T, ruleID, checkName, caseName string, want []annotation) {
	t.Helper()

	caseDir := filepath.Join("golden", checkName, caseName)
	fixture := caseName + ".proto"

	assert.Equal(t, want, runRule(t, ruleID, caseDir, fixture))
}

func runRule(t *testing.T, ruleID, caseDir, fixture string) []annotation {
	t.Helper()

	ctx := context.Background()
	request, err := (&checktest.RequestSpec{
		Files: &checktest.ProtoFileSpec{
			DirPaths:  []string{caseDir, protosDir},
			FilePaths: []string{fixture},
		},
		RuleIDs: []string{ruleID},
	}).ToRequest(ctx)
	require.NoError(t, err)

	client, err := check.NewClientForSpec(&check.Spec{Rules: All})
	require.NoError(t, err)

	response, err := client.Check(ctx, request)
	require.NoError(t, err)

	// Sort on the full location so the order is stable even when one rule fires
	// on several messages in a single file.
	annotations := response.Annotations()
	sort.Slice(annotations, func(i, j int) bool {
		return check.CompareAnnotations(annotations[i], annotations[j]) < 0
	})

	var got []annotation
	for _, a := range annotations {
		got = append(got, annotation{
			ruleID: a.RuleID(),
			file:   a.FileLocation().FileDescriptor().ProtoreflectFileDescriptor().Path(),
		})
	}
	return got
}
