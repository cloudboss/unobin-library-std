package std

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudboss/unobin/pkg/runtime"
	"github.com/cloudboss/unobin/pkg/sdk/state"
	"github.com/stretchr/testify/require"
)

func archiveInputs(path string) map[string]any {
	return map[string]any{
		"path": path, "source-dir": nil, "source-file": nil,
		"entries": []any{map[string]any{"name": "data.txt", "content": "original"}},
	}
}

func TestInvalidArchiveReplacementPreservesPrior(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(map[string]any, string)
		want   string
	}{
		{"empty path", func(in map[string]any, _ string) { in["path"] = "" }, "path is required"},
		{"invalid path", func(in map[string]any, _ string) { in["path"] = "." }, "path is invalid"},
		{"unsafe entry", func(in map[string]any, _ string) {
			in["entries"] = []any{map[string]any{"name": "../escape", "content": "invalid"}}
		}, "unsafe entry name"},
		{"duplicate entries", func(in map[string]any, _ string) {
			in["entries"] = []any{
				map[string]any{"name": "same", "content": "one"},
				map[string]any{"name": "./same", "content": "two"},
			}
		}, "duplicate entry name"},
		{"empty archive", func(in map[string]any, _ string) { in["entries"] = []any{} },
			"zip would be empty"},
		{"missing source file", func(in map[string]any, dir string) {
			in["entries"], in["source-file"] = nil, filepath.Join(dir, "missing")
		}, "stat source-file"},
		{"missing entry source", func(in map[string]any, dir string) {
			in["entries"] = []any{map[string]any{
				"name": "data", "source-file": filepath.Join(dir, "missing"),
			}}
		}, "stat entries.source-file"},
		{"missing source directory", func(in map[string]any, dir string) {
			in["entries"], in["source-dir"] = nil, filepath.Join(dir, "missing")
		}, "stat source-dir"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			priorPath := filepath.Join(dir, "prior.zip")
			inputs := archiveInputs(priorPath)
			executor := consumerExecutor(t, "archive", t.TempDir(), inputs)
			_, prior := applySavedPlan(t, executor, savedPlan(t, executor))
			priorBytes, err := os.ReadFile(priorPath)
			require.NoError(t, err)
			inputs["path"] = filepath.Join(dir, "new", "desired.zip")
			test.change(inputs, dir)
			plan := savedPlan(t, executor)
			resourceOperation(t, plan, "resource.main", runtime.DecisionReplace)
			_, err = executor.ApplyPlanV2(t.Context(), plan)
			require.ErrorContains(t, err, test.want)
			requireFileContent(t, priorPath, string(priorBytes))
			require.NoDirExists(t, filepath.Join(dir, "new"))
			requirePriorTarget(t, executor, prior)
		})
	}
}

func TestInvalidFileReplacementPreservesPrior(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prior.txt")
	executor := consumerExecutor(t, "file", t.TempDir(), map[string]any{
		"path": path, "content": "original", "mode": int64(0o644),
	})
	_, prior := applySavedPlan(t, executor, savedPlan(t, executor))
	executor.Inputs["path"] = ""
	plan := savedPlan(t, executor)
	resourceOperation(t, plan, "resource.main", runtime.DecisionReplace)
	_, err := executor.ApplyPlanV2(t.Context(), plan)
	require.ErrorContains(t, err, "path is required")
	requireFileContent(t, path, "original")
	requirePriorTarget(t, executor, prior)
}

func TestInvalidRandomReplacementPreservesPrior(t *testing.T) {
	executor := consumerExecutor(t, "random", t.TempDir(), map[string]any{
		"byte-length": int64(8), "keepers": nil, "prefix": nil,
	})
	_, prior := applySavedPlan(t, executor, savedPlan(t, executor))
	executor.Inputs["byte-length"] = int64(0)
	plan := savedPlan(t, executor)
	resourceOperation(t, plan, "resource.main", runtime.DecisionReplace)
	_, err := executor.ApplyPlanV2(t.Context(), plan)
	require.ErrorContains(t, err, "byte-length must be at least 1")
	requirePriorTarget(t, executor, prior)
}

func requirePriorTarget(t *testing.T, executor *runtime.Executor, prior *state.SnapshotV2) {
	t.Helper()
	revision, err := executor.Store.CurrentRev()
	require.NoError(t, err)
	current, err := executor.Store.(state.SnapshotBackendV2).GetV2(revision)
	require.NoError(t, err)
	require.NotNil(t, current.Find("resource.main"))
	require.Equal(t, prior.Find("resource.main").Payload.Resource.Target,
		current.Find("resource.main").Payload.Resource.Target)
}
