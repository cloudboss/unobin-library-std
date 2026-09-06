package std

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudboss/unobin/pkg/encrypters"
	"github.com/cloudboss/unobin/pkg/runtime"
	"github.com/cloudboss/unobin/pkg/sdk/encrypt"
	"github.com/cloudboss/unobin/pkg/state/local"
	"github.com/stretchr/testify/require"
)

func TestCompiledConsumer(t *testing.T) {
	repo, err := os.Getwd()
	require.NoError(t, err)
	dir := t.TempDir()
	moduleDir := strings.TrimSpace(string(runConsumerCommand(t, repo, nil, "go",
		"list", "-m", "-f", "{{.Dir}}", "github.com/cloudboss/unobin")))
	cli := filepath.Join(dir, "unobin")
	runConsumerCommand(t, moduleDir, nil, "go", "build", "-buildvcs=false", "-o", cli, "./cmd/unobin")
	generated := filepath.Join(dir, "generated")
	runConsumerCommand(t, dir, nil, cli, "compile",
		"--path", filepath.Join(repo, "testdata", "ub", "compiled"),
		"--name", "consumer", "--version", "test", "--out", generated, "--build",
		"--replace-go-module", libraryPath+"="+repo, "--replace-unobin", moduleDir)
	binary := filepath.Join(generated, "consumer")
	stack, err := os.ReadFile(filepath.Join(repo, "testdata", "ub", "compiled", "test.ub"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.ub"), stack, 0o600))
	runConsumerCommand(t, dir, nil, binary, "pin", "-c", "test.ub")

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		body, err := io.ReadAll(r.Body)
		if err != nil || r.Method != http.MethodPost || r.Header.Get("X-Consumer") != "std" {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		w.Header()["X-Reply"] = []string{"one", "two"}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(append([]byte("received:"), body...))
	}))
	t.Cleanup(server.Close)
	env := []string{"UB_INPUT_url=" + server.URL}
	for _, invalid := range []struct{ env, message string }{
		{"UB_INPUT_byte_length=0", "byte-length must be at least 1"},
		{"UB_INPUT_source_file=missing.txt", "exactly one"},
	} {
		_, stderr, err := consumerCommand(t, dir, append(env, invalid.env), binary,
			"plan", "-c", "test.ub", "-o", "invalid.ubp")
		require.Error(t, err)
		require.Contains(t, strings.ToLower(stderr), invalid.message)
		require.NoFileExists(t, filepath.Join(dir, "data.txt"))
		require.NoFileExists(t, filepath.Join(dir, "bundle.zip"))
		require.Zero(t, requests.Load())
	}
	runConsumerCommand(t, dir, env, binary, "validate", "-c", "test.ub")
	runConsumerCommand(t, dir, env, binary, "plan", "-c", "test.ub", "-o", "create.ubp")
	plan := openConsumerPlan(t, filepath.Join(dir, "create.ubp"))
	for _, name := range []string{"file", "archive", "id"} {
		resourceOperation(t, plan, "resource."+name, runtime.DecisionCreate)
	}
	require.NoFileExists(t, filepath.Join(dir, "data.txt"))
	require.NoFileExists(t, filepath.Join(dir, "bundle.zip"))
	require.Zero(t, requests.Load())
	runConsumerCommand(t, dir, env, binary, "apply", "--format", "json", "create.ubp")
	require.Equal(t, int32(1), requests.Load())
	encoded := runConsumerCommand(t, dir, env, binary, "output", "--format", "json", "-c", "test.ub")
	var output struct {
		Outputs map[string]map[string]any `json:"outputs"`
	}
	require.NoError(t, json.Unmarshal(encoded, &output))
	for _, fixture := range []struct{ name, path string }{
		{"file", "data.txt"}, {"archive", "bundle.zip"},
	} {
		values := output.Outputs[fixture.name]
		values["size"] = int64(values["size"].(float64))
		requireFilesystemResult(t, fixture.name, filepath.Join(dir, fixture.path), "payload", values)
		info, err := os.Stat(filepath.Join(dir, fixture.path))
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o644), info.Mode().Perm())
	}
	requireIDEncodings(t, output.Outputs["id"], 8, "test-")
	digest := output.Outputs["archive"]["sha256"].(string)
	for name, stdout := range map[string]string{"command": digest, "script": "script"} {
		require.Equal(t, stdout, output.Outputs[name]["stdout"])
		require.Equal(t, "", output.Outputs[name]["stderr"])
		require.Equal(t, float64(0), output.Outputs[name]["exit-code"])
		require.GreaterOrEqual(t, output.Outputs[name]["duration"].(float64), float64(0))
	}
	require.Equal(t, "ready", output.Outputs["wait"]["stdout"])
	require.Equal(t, float64(1), output.Outputs["wait"]["attempts"])
	require.Equal(t, float64(http.StatusCreated), output.Outputs["http"]["status"])
	require.Equal(t, "201 Created", output.Outputs["http"]["status-text"])
	require.Equal(t, "received:"+digest, output.Outputs["http"]["body"])
	headers := output.Outputs["http"]["headers"].(map[string]any)
	require.Equal(t, []any{"one", "two"}, headers["X-Reply"])

	store, err := local.NewStore(filepath.Join(dir, "state"), "consumer", "test", encrypters.Noop{})
	require.NoError(t, err)
	revision, err := store.CurrentRev()
	require.NoError(t, err)
	snapshot, err := store.GetV2(revision)
	require.NoError(t, err)
	for _, entry := range snapshot.Entries {
		if entry.Payload.Resource != nil {
			require.Equal(t, libraryPath, entry.Payload.Resource.Target.Binding.LibraryPath)
		}
		if entry.Payload.Action != nil {
			require.Equal(t, libraryPath, entry.Payload.Action.Binding.LibraryPath)
		}
	}
	runConsumerCommand(t, dir, env, binary, "plan", "-c", "test.ub", "-o", "unchanged.ubp")
	plan = openConsumerPlan(t, filepath.Join(dir, "unchanged.ubp"))
	for _, name := range []string{"command", "script", "wait", "http"} {
		actionOperation(t, plan, "action."+name, runtime.DecisionSkip)
	}
	runConsumerCommand(t, dir, env, binary, "apply", "--format", "json", "unchanged.ubp")
	require.Equal(t, int32(1), requests.Load())
}

func runConsumerCommand(
	t *testing.T, dir string, env []string, name string, args ...string,
) []byte {
	t.Helper()
	stdout, stderr, err := consumerCommand(t, dir, env, name, args...)
	require.NoErrorf(t, err, "%s %v\n%s\n%s", name, args, stdout, stderr)
	return stdout
}

func consumerCommand(
	t *testing.T, dir string, env []string, name string, args ...string,
) ([]byte, string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	cmd.Env = append(cmd.Env, env...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.Output()
	return stdout, stderr.String(), err
}

func openConsumerPlan(t *testing.T, path string) *runtime.PlanFileV2 {
	t.Helper()
	encoded, err := os.ReadFile(path)
	require.NoError(t, err)
	plan, err := runtime.OpenPlanV2(encoded, func(*runtime.StateRef) (encrypt.Encrypter, error) {
		return encrypters.Noop{}, nil
	})
	require.NoError(t, err)
	return &plan
}
