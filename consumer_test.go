package std

import (
	"encoding/base64"
	"encoding/hex"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudboss/unobin/pkg/encrypters"
	"github.com/cloudboss/unobin/pkg/lang/syntax"
	"github.com/cloudboss/unobin/pkg/runtime"
	"github.com/cloudboss/unobin/pkg/sdk/state"
	"github.com/cloudboss/unobin/pkg/state/local"
	"github.com/stretchr/testify/require"
)

const libraryPath = "github.com/cloudboss/unobin-library-std"

func consumerExecutor(
	t *testing.T, fixture, stateDir string, inputs map[string]any,
) *runtime.Executor {
	t.Helper()
	path := filepath.Join("testdata", "ub", fixture, "factory.ub")
	source, err := os.ReadFile(path)
	require.NoError(t, err)
	parsed, err := syntax.ParseSource(path, source)
	require.NoError(t, err)
	require.NotNil(t, parsed.Factory)
	catalog, err := runtime.NewLibraryCatalog([]runtime.LibraryRegistration{
		{LibraryPath: libraryPath, New: Library},
	})
	require.NoError(t, err)
	libraries, err := catalog.Libraries(map[string]string{"std": libraryPath})
	require.NoError(t, err)
	store, err := local.NewStore(stateDir, "consumer", "test", encrypters.Noop{})
	require.NoError(t, err)
	body := &parsed.Factory.Body
	return &runtime.Executor{
		DAG: runtime.BuildSyntaxDAG(*body, libraries), SyntaxSource: body,
		Libraries: libraries, LibraryCatalog: catalog, Inputs: inputs, Store: store,
		Factory: state.FactoryInfo{Name: "consumer", Version: "test", ContentRevision: "test"},
	}
}

func savedPlan(t *testing.T, executor *runtime.Executor) *runtime.PlanFileV2 {
	t.Helper()
	plan, err := executor.PlanV2(t.Context())
	require.NoError(t, err)
	encoded, err := runtime.EncodePlanV2(*plan)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "plan.json")
	require.NoError(t, os.WriteFile(path, encoded, 0o600))
	saved, err := os.ReadFile(path)
	require.NoError(t, err)
	decoded, err := runtime.DecodePlanV2(saved)
	require.NoError(t, err)
	require.Equal(t, 2, decoded.FormatVersion)
	return &decoded
}

func resourceOperation(
	t *testing.T, plan *runtime.PlanFileV2, address string, decision runtime.Decision,
) *runtime.ResourcePlanOperation {
	t.Helper()
	for _, step := range plan.Steps {
		if step.Address == address {
			require.NotNil(t, step.Operation.Resource)
			require.Equal(t, decision, step.Operation.Resource.Decision)
			return step.Operation.Resource
		}
	}
	t.Fatalf("missing resource operation %s", address)
	return nil
}

func applySavedPlan(
	t *testing.T, executor *runtime.Executor, plan *runtime.PlanFileV2,
) (*runtime.ExecResult, *state.SnapshotV2) {
	t.Helper()
	result, err := executor.ApplyPlanV2(t.Context(), plan)
	require.NoError(t, err)
	require.NotEmpty(t, result.WrittenRev)
	store := executor.Store.(state.SnapshotBackendV2)
	snapshot, err := store.GetV2(result.WrittenRev)
	require.NoError(t, err)
	require.NoError(t, snapshot.Validate())
	require.Equal(t, 2, snapshot.FormatVersion)
	return result, snapshot
}

func requireFileContent(t *testing.T, path, content string) {
	t.Helper()
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, content, string(got))
}

func TestFileSavedPlanLifecycle(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first", "file.txt")
	second := filepath.Join(dir, "second", "file.txt")
	executor := consumerExecutor(t, "file", t.TempDir(), map[string]any{
		"path": first, "content": "first", "mode": int64(0o644),
	})
	plan := savedPlan(t, executor)
	resourceOperation(t, plan, "resource.main", runtime.DecisionCreate)
	require.NoDirExists(t, filepath.Dir(first))
	result, snapshot := applySavedPlan(t, executor, plan)
	requireFileContent(t, first, "first")
	require.Equal(t, int64(5), result.Outputs["size"])
	target := snapshot.Find("resource.main").Payload.Resource.Target
	require.Equal(t, runtime.Binding{LibraryPath: libraryPath, Export: "fs-file"}, target.Binding)
	require.Equal(t, 1, target.SchemaVersion)
	require.Equal(t, 1, target.Identity.Version)
	require.Nil(t, target.Identity.StableID)

	plan = savedPlan(t, executor)
	resourceOperation(t, plan, "resource.main", runtime.DecisionNoOp)
	_, unchanged := applySavedPlan(t, executor, plan)
	require.Equal(t, target, unchanged.Find("resource.main").Payload.Resource.Target)

	executor.Inputs["path"], executor.Inputs["content"] = second, "second"
	plan = savedPlan(t, executor)
	op := resourceOperation(t, plan, "resource.main", runtime.DecisionReplace)
	require.Equal(t, []string{"address:path"}, op.Reasons)
	requireFileContent(t, first, "first")
	require.NoDirExists(t, filepath.Dir(second))
	applySavedPlan(t, executor, plan)
	require.NoFileExists(t, first)
	requireFileContent(t, second, "second")
}

func TestRandomSavedPlanLifecycle(t *testing.T) {
	stateDir := t.TempDir()
	inputs := map[string]any{
		"byte-length": int64(8), "keepers": map[string]any{"a": "1", "b": "2"},
		"prefix": "before-",
	}
	executor := consumerExecutor(t, "random", stateDir, inputs)
	plan := savedPlan(t, executor)
	resourceOperation(t, plan, "resource.main", runtime.DecisionCreate)
	result, snapshot := applySavedPlan(t, executor, plan)
	requireIDEncodings(t, result.Outputs, 8, "before-")
	target := snapshot.Find("resource.main").Payload.Resource.Target
	require.NotNil(t, target.Identity.StableID)
	require.Equal(t, result.Outputs["id"], *target.Identity.StableID)

	inputs["keepers"] = map[string]any{"b": "2", "a": "1"}
	executor = consumerExecutor(t, "random", stateDir, inputs)
	plan = savedPlan(t, executor)
	resourceOperation(t, plan, "resource.main", runtime.DecisionNoOp)
	unchanged, _ := applySavedPlan(t, executor, plan)
	require.Equal(t, result.Outputs, unchanged.Outputs)

	inputs["prefix"] = "after-"
	plan = savedPlan(t, executor)
	op := resourceOperation(t, plan, "resource.main", runtime.DecisionReplace)
	require.Equal(t, []string{"input:prefix"}, op.Reasons)
	result, snapshot = applySavedPlan(t, executor, plan)
	requireIDEncodings(t, result.Outputs, 8, "after-")
	require.Equal(t, result.Outputs["id"],
		*snapshot.Find("resource.main").Payload.Resource.Target.Identity.StableID)
}

func requireIDEncodings(t *testing.T, outputs map[string]any, length int, prefix string) {
	t.Helper()
	id, ok := outputs["id"].(string)
	require.True(t, ok)
	require.NotEmpty(t, id)
	raw, err := base64.RawURLEncoding.DecodeString(id)
	require.NoError(t, err)
	require.Len(t, raw, length)
	require.Equal(t, map[string]any{
		"id": id, "b64-url": prefix + id,
		"b64-std": prefix + base64.StdEncoding.EncodeToString(raw),
		"dec":     prefix + new(big.Int).SetBytes(raw).String(),
		"hex":     prefix + hex.EncodeToString(raw),
	}, outputs)
}
