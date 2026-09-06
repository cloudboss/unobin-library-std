package std

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudboss/unobin/pkg/runtime"
	"github.com/stretchr/testify/require"
)

func TestFilesystemSavedPlanChanges(t *testing.T) {
	for _, fixture := range []string{"file", "archive"} {
		t.Run(fixture, func(t *testing.T) {
			dir, stateDir := t.TempDir(), t.TempDir()
			path := filepath.Join(dir, "first", "target")
			inputs := filesystemInputs(fixture, path, "first")
			executor := consumerExecutor(t, fixture, stateDir, inputs)
			plan := savedPlan(t, executor)
			resourceOperation(t, plan, "resource.main", runtime.DecisionCreate)
			require.NoDirExists(t, filepath.Dir(path))
			result, prior := applySavedPlan(t, executor, plan)
			requireFilesystemResult(t, fixture, path, "first", result.Outputs)

			plan = savedPlan(t, executor)
			resourceOperation(t, plan, "resource.main", runtime.DecisionNoOp)
			_, snapshot := applySavedPlan(t, executor, plan)
			require.Equal(t, prior.Find("resource.main").Payload,
				snapshot.Find("resource.main").Payload)

			setFilesystemContent(fixture, inputs, "second")
			plan = savedPlan(t, executor)
			resourceOperation(t, plan, "resource.main", runtime.DecisionUpdate)
			requirePendingOutputs(t, plan)
			requireFilesystemResult(t, fixture, path, "first", result.Outputs)
			result, _ = applySavedPlan(t, executor, plan)
			requireFilesystemResult(t, fixture, path, "second", result.Outputs)

			if fixture == "file" {
				inputs["mode"] = int64(0o600)
				plan = savedPlan(t, executor)
				resourceOperation(t, plan, "resource.main", runtime.DecisionUpdate)
				result, _ = applySavedPlan(t, executor, plan)
				info, err := os.Stat(path)
				require.NoError(t, err)
				require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
				requireFilesystemResult(t, fixture, path, "second", result.Outputs)
			}

			oldPath := path
			path = filepath.Join(dir, "new", "target")
			inputs["path"] = path
			setFilesystemContent(fixture, inputs, "replacement")
			plan = savedPlan(t, executor)
			op := resourceOperation(t, plan, "resource.main", runtime.DecisionReplace)
			require.Equal(t, []string{"address:path"}, op.Reasons)
			require.NoDirExists(t, filepath.Dir(path))
			result, _ = applySavedPlan(t, executor, plan)
			require.NoFileExists(t, oldPath)
			requireFilesystemResult(t, fixture, path, "replacement", result.Outputs)

			require.NoError(t, os.Remove(path))
			setFilesystemContent(fixture, inputs, "recreated")
			plan = savedPlan(t, executor)
			op = resourceOperation(t, plan, "resource.main", runtime.DecisionCreate)
			require.Equal(t, []string{"remote-missing"}, op.Reasons)
			require.NoFileExists(t, path)
			result, _ = applySavedPlan(t, executor, plan)
			requireFilesystemResult(t, fixture, path, "recreated", result.Outputs)

			require.NoError(t, os.WriteFile(path, []byte("external edit"), 0o600))
			plan = savedPlan(t, executor)
			op = resourceOperation(t, plan, "resource.main", runtime.DecisionUpdate)
			require.Empty(t, op.Reasons)
			requireFileContent(t, path, "external edit")
			result, _ = applySavedPlan(t, executor, plan)
			requireFilesystemResult(t, fixture, path, "recreated", result.Outputs)

			executor = consumerExecutor(t, "empty", stateDir, nil)
			plan = savedPlan(t, executor)
			op = resourceOperation(t, plan, "resource.main", runtime.DecisionDestroy)
			require.Equal(t, libraryPath, op.Prior.Binding.LibraryPath)
			_, snapshot = applySavedPlan(t, executor, plan)
			require.NoFileExists(t, path)
			require.Nil(t, snapshot.Find("resource.main"))
		})
	}
}

func TestFilesystemSavedDestroy(t *testing.T) {
	for _, fixture := range []string{"file", "archive"} {
		for _, absent := range []bool{false, true} {
			t.Run(fixture+map[bool]string{true: "/absent", false: "/present"}[absent],
				func(t *testing.T) {
					path := filepath.Join(t.TempDir(), "target")
					executor := consumerExecutor(t, fixture, t.TempDir(),
						filesystemInputs(fixture, path, "data"))
					applySavedPlan(t, executor, savedPlan(t, executor))
					if absent {
						require.NoError(t, os.Remove(path))
					}
					executor.Destroy = true
					plan := savedPlan(t, executor)
					resourceOperation(t, plan, "resource.main", runtime.DecisionDestroy)
					_, snapshot := applySavedPlan(t, executor, plan)
					require.NoFileExists(t, path)
					require.Empty(t, snapshot.Entries)
				})
		}
	}
}

func TestArchiveSourcePathUpdates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "archive.zip")
	inputs := archiveInputs(path)
	inputs["entries"] = nil
	executor := consumerExecutor(t, "archive", t.TempDir(), inputs)
	for i, content := range []string{"first", "second"} {
		source := filepath.Join(dir, content, "data.txt")
		require.NoError(t, os.MkdirAll(filepath.Dir(source), 0o755))
		require.NoError(t, os.WriteFile(source, []byte(content), 0o600))
		inputs["source-file"] = source
		plan := savedPlan(t, executor)
		decision := []runtime.Decision{runtime.DecisionCreate, runtime.DecisionUpdate}[i]
		resourceOperation(t, plan, "resource.main", decision)
		requirePendingOutputs(t, plan)
		result, _ := applySavedPlan(t, executor, plan)
		requireFilesystemResult(t, "archive", path, content, result.Outputs)
	}
}

func filesystemInputs(fixture, path, content string) map[string]any {
	if fixture == "archive" {
		inputs := archiveInputs(path)
		setFilesystemContent(fixture, inputs, content)
		return inputs
	}
	return map[string]any{"path": path, "content": content, "mode": int64(0o644)}
}

func setFilesystemContent(fixture string, inputs map[string]any, content string) {
	if fixture == "archive" {
		inputs["entries"] = []any{map[string]any{"name": "data.txt", "content": content}}
	} else {
		inputs["content"] = content
	}
}

func requireFilesystemResult(
	t *testing.T, fixture, path, content string, outputs map[string]any,
) {
	t.Helper()
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	digest := sha256.Sum256(body)
	expected := map[string]any{
		"sha256": hex.EncodeToString(digest[:]), "size": int64(len(body)),
	}
	if fixture == "archive" {
		expected["base64-sha256"] = base64.StdEncoding.EncodeToString(digest[:])
		reader, err := zip.OpenReader(path)
		require.NoError(t, err)
		defer func() { require.NoError(t, reader.Close()) }()
		members := make(map[string]string)
		for _, member := range reader.File {
			file, err := member.Open()
			require.NoError(t, err)
			bytes, err := io.ReadAll(file)
			require.NoError(t, err)
			require.NoError(t, file.Close())
			members[member.Name] = string(bytes)
		}
		require.Equal(t, map[string]string{"data.txt": content}, members)
	} else {
		require.Equal(t, content, string(body))
	}
	require.Equal(t, expected, outputs)
}

func requirePendingOutputs(t *testing.T, plan *runtime.PlanFileV2) {
	t.Helper()
	found := false
	for _, step := range plan.Steps {
		if step.Operation.Output != nil {
			found = true
			require.Truef(t, step.Operation.Output.Value.HasPending(), "%s is not pending", step.Address)
		}
	}
	require.True(t, found)
}
