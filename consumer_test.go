package std

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudboss/unobin/pkg/lang/syntax"
	"github.com/cloudboss/unobin/pkg/runtime"
	"github.com/cloudboss/unobin/pkg/sdk/encrypt"
	"github.com/cloudboss/unobin/pkg/sdk/state"
	"github.com/cloudboss/unobin/pkg/state/local"
	"github.com/stretchr/testify/require"
)

const libraryPath = "github.com/cloudboss/unobin-library-std"

type testEncrypter struct{}

func (testEncrypter) Encrypt(plaintext []byte) ([]byte, error)  { return plaintext, nil }
func (testEncrypter) Decrypt(ciphertext []byte) ([]byte, error) { return ciphertext, nil }
func (testEncrypter) Describe() encrypt.Description {
	return encrypt.Description{KeySource: "test"}
}

func consumerExecutor(
	t *testing.T,
	fixture string,
	stateDir string,
	inputs map[string]any,
) *runtime.Executor {
	t.Helper()
	path := filepath.Join("testdata", "ub", fixture, "factory.ub")
	source, err := os.ReadFile(path)
	require.NoError(t, err)
	parsed, err := syntax.ParseSource(path, source)
	require.NoError(t, err)
	require.NotNil(t, parsed.Factory)

	library := runtime.LibraryWithPath(Library(), libraryPath)
	libraries := make(map[string]*runtime.Library, len(parsed.Factory.Body.Imports))
	for _, imported := range parsed.Factory.Body.Imports {
		require.Equal(t, libraryPath, imported.Ref.Value)
		libraries[imported.Alias.Name] = library
	}
	store, err := local.NewStore(stateDir, "consumer", "test", testEncrypter{})
	require.NoError(t, err)
	body := &parsed.Factory.Body
	return &runtime.Executor{
		DAG:          runtime.BuildSyntaxDAG(*body, libraries),
		SyntaxSource: body,
		Libraries:    libraries,
		Inputs:       inputs,
		Store:        store,
		Factory: state.FactoryInfo{
			Name: "consumer", Version: "test", ContentRevision: "test",
		},
	}
}

func savedPlan(t *testing.T, executor *runtime.Executor) *runtime.PlanFile {
	t.Helper()
	plan, err := executor.Plan(t.Context())
	require.NoError(t, err)
	encoded, err := runtime.EncodePlan(plan)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "plan.json")
	require.NoError(t, os.WriteFile(path, encoded, 0o600))
	saved, err := os.ReadFile(path)
	require.NoError(t, err)
	decoded, err := runtime.DecodePlan(saved)
	require.NoError(t, err)
	require.Equal(t, runtime.PlanFormatVersion, decoded.FormatVersion)
	return decoded
}

func resourceStep(
	t *testing.T,
	plan *runtime.PlanFile,
	address string,
	decision runtime.Decision,
) *runtime.PlanStep {
	t.Helper()
	for i := range plan.Steps {
		step := &plan.Steps[i]
		if step.Address == address {
			require.Equal(t, decision, step.Decision)
			return step
		}
	}
	t.Fatalf("missing resource step %s", address)
	return nil
}

func applySavedPlan(
	t *testing.T,
	executor *runtime.Executor,
	plan *runtime.PlanFile,
) (*runtime.ExecResult, *state.Snapshot) {
	t.Helper()
	result, err := executor.ApplyPlan(t.Context(), plan)
	require.NoError(t, err)
	require.NotEmpty(t, result.WrittenRev)
	snapshot, err := executor.Store.Current()
	require.NoError(t, err)
	require.NoError(t, snapshot.Validate())
	require.Equal(t, state.CurrentFormatVersion, snapshot.FormatVersion)
	return result, snapshot
}

func requireFileContent(t *testing.T, path, content string) {
	t.Helper()
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, content, string(got))
}
