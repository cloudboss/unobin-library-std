package std

import (
	"crypto/rand"
	"sync/atomic"
	"testing"

	"github.com/cloudboss/unobin/pkg/runtime"
	"github.com/stretchr/testify/require"
)

type countingEntropy struct {
	calls atomic.Uint32
}

func (r *countingEntropy) Read(bytes []byte) (int, error) {
	r.calls.Add(1)
	for i := range bytes {
		bytes[i] = 0x2a
	}
	return len(bytes), nil
}

func TestRandomReplacementRulesGenerateOnlyDuringApply(t *testing.T) {
	// The entropy reader is process-global, so this test must remain serial.
	previous := rand.Reader
	entropy := &countingEntropy{}
	rand.Reader = entropy
	t.Cleanup(func() { rand.Reader = previous })

	for _, test := range []struct {
		name   string
		field  string
		before any
		after  any
		length int
		prefix string
	}{
		{"byte length", "byte-length", int64(1), int64(8), 8, ""},
		{"prefix", "prefix", "old-", "new-", 1, "new-"},
		{"keeper value", "keepers", map[string]any{"a": "1"}, map[string]any{"a": "2"}, 1, ""},
		{"keeper added", "keepers", map[string]any{}, map[string]any{"a": "1"}, 1, ""},
		{"keeper removed", "keepers", map[string]any{"a": "1"}, map[string]any{}, 1, ""},
		{"keepers null to empty", "keepers", nil, map[string]any{}, 1, ""},
		{"keepers empty to null", "keepers", map[string]any{}, nil, 1, ""},
		{"prefix null to empty", "prefix", nil, "", 1, ""},
		{"prefix empty to null", "prefix", "", nil, 1, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			stateDir := t.TempDir()
			inputs := map[string]any{"byte-length": int64(1), "keepers": nil, "prefix": nil}
			inputs[test.field] = test.before
			executor := consumerExecutor(t, "random", stateDir, inputs)
			before := entropy.calls.Load()
			plan := savedPlan(t, executor)
			resourceOperation(t, plan, "resource.main", runtime.DecisionCreate)
			requirePendingOutputs(t, plan)
			require.Equal(t, before, entropy.calls.Load())
			applySavedPlan(t, executor, plan)
			require.Equal(t, before+1, entropy.calls.Load())

			inputs[test.field] = test.after
			plan = savedPlan(t, executor)
			op := resourceOperation(t, plan, "resource.main", runtime.DecisionReplace)
			require.Equal(t, []string{"input:" + test.field}, op.Reasons)
			requirePendingOutputs(t, plan)
			require.Equal(t, before+1, entropy.calls.Load())
			result, snapshot := applySavedPlan(t, executor, plan)
			require.Equal(t, before+2, entropy.calls.Load())
			requireIDEncodings(t, result.Outputs, test.length, test.prefix)
			target := snapshot.Find("resource.main").Payload.Resource.Target
			require.Equal(t, result.Outputs["id"], *target.Identity.StableID)

			executor = consumerExecutor(t, "random", stateDir, inputs)
			plan = savedPlan(t, executor)
			resourceOperation(t, plan, "resource.main", runtime.DecisionNoOp)
			unchanged, _ := applySavedPlan(t, executor, plan)
			require.Equal(t, result.Outputs, unchanged.Outputs)
			require.Equal(t, before+2, entropy.calls.Load())
		})
	}
}
