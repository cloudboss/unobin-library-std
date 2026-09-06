package std

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cloudboss/unobin/pkg/codegen"
	"github.com/stretchr/testify/require"
)

func TestConsumerWithoutModuleReplacements(t *testing.T) {
	repo, err := os.Getwd()
	require.NoError(t, err)
	dir := t.TempDir()
	buildConsumer(t, repo, dir)
	generated := filepath.Join(dir, "generated")
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
		"-require="+libraryPath+"@"+version)
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
	require.Equal(t, "v0.12.0-a.2", selected["github.com/cloudboss/unobin"])

	revision, err := codegen.ContentRevision(generated)
	require.NoError(t, err)
	flags := "-X main.factoryVersion=test -X main.contentRevision=" + revision +
		" -X main.unobinVersion=" + selected["github.com/cloudboss/unobin"]
	binary := filepath.Join(generated, "consumer-distribution")
	runConsumerCommand(t, generated, nil, "go", "build", "-buildvcs=false",
		"-ldflags", flags, "-o", binary, ".")
	checkCompiledConsumer(t, repo, dir, binary)
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
