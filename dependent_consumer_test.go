package std

import (
	"path/filepath"
	"testing"

	"github.com/cloudboss/unobin/pkg/runtime"
	"github.com/stretchr/testify/require"
)

func TestSavedPlanResolvesDependentOutputsAndTriggers(t *testing.T) {
	dir := t.TempDir()
	inputs := map[string]any{
		"archive-path": filepath.Join(dir, "archive.zip"),
		"digest-path":  filepath.Join(dir, "digest.txt"),
		"id-path":      filepath.Join(dir, "id.txt"),
		"log-path":     filepath.Join(dir, "runs.txt"),
		"content":      "first", "prefix": "old-",
	}
	executor := consumerExecutor(t, "dependent", t.TempDir(), inputs)
	require.Same(t, executor.Libraries["std"], executor.Libraries["other"])
	plan := savedPlan(t, executor)
	for _, field := range []string{"archive-path", "digest-path", "id-path", "log-path"} {
		require.NoFileExists(t, inputs[field].(string))
	}
	first, snapshot := applySavedPlan(t, executor, plan)
	requireFileContent(t, inputs["digest-path"].(string), first.Outputs["digest"].(string))
	requireFileContent(t, inputs["id-path"].(string), first.Outputs["id"].(string))
	requireFileContent(t, inputs["log-path"].(string), first.Outputs["digest"].(string))
	for _, address := range []string{"resource.digest", "resource.encoded"} {
		target := snapshot.Find(address).Payload.Resource.Target
		require.Equal(t, runtime.Binding{LibraryPath: libraryPath, Export: "fs-file"}, target.Binding)
		require.Equal(t, libraryPath, target.Configuration.LibraryPath)
	}
	firstTrigger := snapshot.Find("action.record").Payload.Action.TriggerHash
	require.NotEmpty(t, firstTrigger)

	inputs["content"], inputs["prefix"] = "second", "new-"
	plan = savedPlan(t, executor)
	resourceOperation(t, plan, "resource.archive", runtime.DecisionUpdate)
	resourceOperation(t, plan, "resource.id", runtime.DecisionReplace)
	for _, address := range []string{"resource.digest", "resource.encoded"} {
		op := resourceOperation(t, plan, address, runtime.DecisionUpdate)
		fields, ok := op.Desired.Inputs.ObjectFields()
		require.True(t, ok)
		require.True(t, fields["content"].HasPending())
	}
	action := actionOperation(t, plan, "action.record", runtime.DecisionRerun)
	require.Empty(t, action.Desired.TriggerHash)
	requireFileContent(t, inputs["log-path"].(string), first.Outputs["digest"].(string))
	second, snapshot := applySavedPlan(t, executor, plan)
	requireFileContent(t, inputs["digest-path"].(string), second.Outputs["digest"].(string))
	requireFileContent(t, inputs["id-path"].(string), second.Outputs["id"].(string))
	log := first.Outputs["digest"].(string) + second.Outputs["digest"].(string)
	requireFileContent(t, inputs["log-path"].(string), log)
	secondTrigger := snapshot.Find("action.record").Payload.Action.TriggerHash
	require.NotEmpty(t, secondTrigger)
	require.NotEqual(t, firstTrigger, secondTrigger)

	plan = savedPlan(t, executor)
	actionOperation(t, plan, "action.record", runtime.DecisionSkip)
	unchanged, _ := applySavedPlan(t, executor, plan)
	require.Equal(t, second.Outputs, unchanged.Outputs)
	requireFileContent(t, inputs["log-path"].(string), log)
}

func actionOperation(
	t *testing.T, plan *runtime.PlanFileV2, address string, decision runtime.Decision,
) *runtime.ActionPlanOperation {
	t.Helper()
	for _, step := range plan.Steps {
		if step.Address == address {
			require.NotNil(t, step.Operation.Action)
			require.Equal(t, decision, step.Operation.Action.Decision)
			return step.Operation.Action
		}
	}
	t.Fatalf("missing action operation %s", address)
	return nil
}
