//go:build release

package std

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cloudboss/unobin/pkg/codegen"
	"github.com/cloudboss/unobin/pkg/compile"
	"github.com/stretchr/testify/require"
)

func TestConsumerWithoutModuleReplacements(t *testing.T) {
	if testing.Short() {
		t.Skip("skipped: downloads dependencies and builds a consumer factory")
	}
	repo, err := os.Getwd()
	require.NoError(t, err)
	dir := t.TempDir()
	generated := filepath.Join(dir, "generated")
	moduleDir := strings.TrimSpace(string(runConsumerCommand(t, repo, nil, "go",
		"list", "-m", "-f", "{{.Dir}}", "github.com/cloudboss/unobin")))
	fixture := filepath.Join(repo, "testdata", "ub", "compiled", "valid", "all-exports")
	var diagnostics bytes.Buffer
	require.NoError(t, compile.Run(compile.Options{
		FactoryPath: filepath.Join(fixture, "src"), OutDir: generated,
		StackName: "consumer", LibraryPath: "example.com/std/consumer", Version: "v0.0.0",
		GoVersion: compile.GoMajorMinor(), CLIVersion: "v0.12.0-a.3",
		ReplaceUnobin: moduleDir, ReplaceGoModules: map[string]string{libraryPath: repo},
		Stdout: &diagnostics, Stderr: &diagnostics,
	}), diagnostics.String())
	version := "v0.5.0-a.1"
	proxy := createModuleProxy(t, repo, version)
	upstream := strings.TrimSpace(string(runConsumerCommand(t, repo, nil, "go", "env", "GOPROXY")))
	t.Setenv("GOPROXY", (&url.URL{Scheme: "file", Path: proxy}).String()+","+upstream)
	t.Setenv("GONOSUMDB", libraryPath)
	t.Setenv("GONOPROXY", "none")
	t.Setenv("GOMODCACHE", filepath.Join(t.TempDir(), "modules"))
	t.Setenv("GOFLAGS", strings.TrimSpace(os.Getenv("GOFLAGS")+" -modcacherw"))
	runConsumerCommand(t, generated, nil, "go", "mod", "edit",
		"-dropreplace="+libraryPath, "-dropreplace=github.com/cloudboss/unobin",
		"-require="+libraryPath+"@"+version, "-require=github.com/cloudboss/unobin@v0.12.0-a.3")
	runConsumerCommand(t, generated, nil, "go", "mod", "tidy")

	modules := runConsumerCommand(t, generated, nil, "go", "list", "-m", "-json", "all")
	decoder := json.NewDecoder(bytes.NewReader(modules))
	selected := make(map[string]string)
	for {
		var module struct {
			Path    string
			Version string
			Replace json.RawMessage
		}
		err := decoder.Decode(&module)
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		require.Empty(t, module.Replace, module.Path)
		selected[module.Path] = module.Version
	}
	require.Equal(t, version, selected[libraryPath])
	require.Equal(t, "v0.12.0-a.3", selected["github.com/cloudboss/unobin"])

	revision, err := codegen.ContentRevision(generated)
	require.NoError(t, err)
	flags := "-X main.factoryVersion=v0.0.0 -X main.contentRevision=" + revision +
		" -X main.unobinVersion=" + selected["github.com/cloudboss/unobin"]
	binary := filepath.Join(generated, "consumer-distribution")
	runConsumerCommand(t, generated, nil, "go", "build", "-buildvcs=false",
		"-ldflags", flags, "-o", binary, ".")
	stack, err := os.ReadFile(filepath.Join(fixture, "test.ub"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.ub"), stack, 0o600))
	checkCompiledConsumer(t, func(url string) {
		env := []string{"STD_CONSUMER_URL=" + url}
		runConsumerCommand(t, dir, env, binary, "pin", "-c", "test.ub")
		runConsumerCommand(t, dir, env, binary, "validate", "-c", "test.ub")
		for _, plan := range []string{"create.ubp", "unchanged.ubp"} {
			runConsumerCommand(t, dir, env, binary, "plan", "-c", "test.ub", "-o", plan)
			runConsumerCommand(t, dir, env, binary, "apply", "--format", "json", plan)
		}
		encoded := runConsumerCommand(t, dir, env, binary, "output", "--format", "json", "-c", "test.ub")
		expected, err := os.ReadFile(filepath.Join(fixture, "want", "output.json"))
		require.NoError(t, err)
		var got, want struct {
			Outputs map[string]any `json:"outputs"`
		}
		require.NoError(t, json.Unmarshal(encoded, &got))
		require.NoError(t, json.Unmarshal(expected, &want))
		require.Equal(t, want.Outputs, got.Outputs)
	})
}

func runConsumerCommand(
	t *testing.T, dir string, env []string, name string, args ...string,
) []byte {
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
	require.NoErrorf(t, err, "%s %v\n%s\n%s", name, args, stdout, stderr.String())
	return stdout
}

func createModuleProxy(t *testing.T, repo, version string) string {
	t.Helper()
	root := t.TempDir()
	versions := filepath.Join(root, filepath.FromSlash(libraryPath), "@v")
	require.NoError(t, os.MkdirAll(versions, 0o755))
	mod, err := os.ReadFile(filepath.Join(repo, "go.mod"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(versions, version+".mod"), mod, 0o600))
	info, err := json.Marshal(map[string]string{
		"Version": version, "Time": "2026-09-06T00:00:00Z",
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(versions, version+".info"), info, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(versions, "list"), []byte(version+"\n"), 0o600))
	files := []string{"go.mod", "go.sum", "doc.go", "library.go"}
	err = filepath.WalkDir(filepath.Join(repo, "internal"),
		func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			relative, err := filepath.Rel(repo, path)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(relative))
			return nil
		})
	require.NoError(t, err)
	slices.Sort(files)
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	for _, name := range files {
		body, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(name)))
		require.NoError(t, err)
		writer, err := archive.Create(fmt.Sprintf("%s@%s/%s", libraryPath, version, name))
		require.NoError(t, err)
		_, err = writer.Write(body)
		require.NoError(t, err)
	}
	require.NoError(t, archive.Close())
	require.NoError(t, os.WriteFile(filepath.Join(versions, version+".zip"), buffer.Bytes(), 0o600))
	return root
}
