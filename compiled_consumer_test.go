package std

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/cloudboss/unobin/pkg/e2etest"
	"github.com/stretchr/testify/require"
)

func TestCompiledConsumer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipped: builds a consumer factory")
	}
	checkCompiledConsumer(t, func(url string) {
		t.Run("cases", func(t *testing.T) {
			e2etest.RunCompiledCases(t, "testdata/ub/compiled/valid",
				e2etest.WithGoModule(libraryPath, "."),
				e2etest.WithEnv(map[string]string{"STD_CONSUMER_URL": url}),
			)
		})
	})
}

func checkCompiledConsumer(t *testing.T, run func(string)) {
	t.Helper()
	type observation struct {
		outputs   map[string]map[string]any
		directory string
		files     map[string][]byte
		modes     map[string]os.FileMode
	}
	results := make(chan observation, 1)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != http.MethodPost || r.Header.Get("X-Consumer") != "std" {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		got := observation{files: map[string][]byte{}, modes: map[string]os.FileMode{}}
		if err := json.NewDecoder(r.Body).Decode(&got.outputs); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		stdout, ok := got.outputs["command"]["stdout"].(string)
		if !ok {
			http.Error(w, "missing command output", http.StatusBadRequest)
			return
		}
		_, got.directory, ok = strings.Cut(stdout, "\n")
		if !ok {
			http.Error(w, "missing command working directory", http.StatusBadRequest)
			return
		}
		// Capture files before the framework removes the completed case's workspace.
		for name, path := range map[string]string{"file": "data.txt", "archive": "bundle.zip"} {
			body, err := os.ReadFile(filepath.Join(got.directory, path))
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			info, err := os.Stat(filepath.Join(got.directory, path))
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			got.files[name], got.modes[name] = body, info.Mode().Perm()
		}
		select {
		case results <- got:
		default:
			http.Error(w, "action ran again", http.StatusConflict)
			return
		}
		w.Header()["Date"] = nil
		w.Header()["X-Reply"] = []string{"one", "two"}
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, "received:"+got.outputs["archive"]["sha256"].(string))
	}))
	t.Cleanup(server.Close)
	run(server.URL)
	require.Equal(t, int32(1), requests.Load())
	var got observation
	select {
	case got = <-results:
	default:
		t.Fatal("consumer did not send its outputs and files")
	}
	for _, name := range []string{"file", "archive"} {
		values := got.outputs[name]
		values["size"] = int64(values["size"].(float64))
		requireFilesystemBytes(t, name, got.files[name], "payload", values)
		require.Equal(t, os.FileMode(0o644), got.modes[name])
	}
	requireIDEncodings(t, got.outputs["id"], 8, "test-")
	digest := got.outputs["archive"]["sha256"].(string)
	for name, stdout := range map[string]string{
		"command": digest + "\n" + got.directory, "script": server.URL,
	} {
		require.Equal(t, stdout, got.outputs[name]["stdout"])
		require.Equal(t, "", got.outputs[name]["stderr"])
		require.Equal(t, float64(0), got.outputs[name]["exit-code"])
		require.GreaterOrEqual(t, got.outputs[name]["duration"].(float64), float64(0))
	}
	require.Equal(t, "ready", got.outputs["wait"]["stdout"])
	require.Equal(t, "", got.outputs["wait"]["stderr"])
	require.Equal(t, float64(1), got.outputs["wait"]["attempts"])
	require.GreaterOrEqual(t, got.outputs["wait"]["duration"].(float64), float64(0))
}
