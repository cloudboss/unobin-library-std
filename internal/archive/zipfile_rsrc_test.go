package archive

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudboss/unobin/pkg/runtime"
	"github.com/stretchr/testify/require"
)

type zipMember struct {
	body string
	mode os.FileMode
}

func TestZipFileCreateFromEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bundle.zip")
	contentA := "one"
	contentB := "two"
	mode := int64(0o600)

	out, err := (&ZipFile{
		Path: path,
		Mode: 0o644,
		Entries: &[]ZipEntry{
			{Name: "b.txt", Content: &contentB},
			{Name: "a.txt", Content: &contentA, Mode: &mode},
		},
	}).Create(context.Background(), runtime.NoConfig{})
	require.NoError(t, err)
	require.NotNil(t, out)
	require.Len(t, out.SHA256, 64)
	require.NotEmpty(t, out.Base64SHA256)
	require.Positive(t, out.Size)

	members := readZipMembers(t, path)
	require.Equal(t, map[string]zipMember{
		"a.txt": {body: "one", mode: 0o600},
		"b.txt": {body: "two", mode: 0o644},
	}, members)
}

func TestZipFileCreateFromDirectory(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	require.NoError(t, os.MkdirAll(filepath.Join(src, "nested"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "keep.txt"), []byte("keep"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(src, "skip.txt"), []byte("skip"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(src, "nested", "item.txt"), []byte("item"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(src, "nested", "skip-log"), []byte("skip"), 0o644))

	excludes := []string{"skip.txt", "nested/skip-*"}
	out, err := (&ZipFile{
		Path:      filepath.Join(dir, "out.zip"),
		Mode:      0o644,
		SourceDir: &src,
		Excludes:  &excludes,
	}).Create(context.Background(), runtime.NoConfig{})
	require.NoError(t, err)
	require.NotNil(t, out)

	members := readZipMembers(t, filepath.Join(dir, "out.zip"))
	require.Equal(t, map[string]zipMember{
		"keep.txt":        {body: "keep", mode: 0o644},
		"nested/item.txt": {body: "item", mode: 0o600},
	}, members)
}

func TestZipFileCreateFromFile(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "app")
	require.NoError(t, os.WriteFile(source, []byte("binary"), 0o755))
	name := "bin/app"

	_, err := (&ZipFile{
		Path:       filepath.Join(dir, "app.zip"),
		Mode:       0o644,
		SourceFile: &source,
		EntryName:  &name,
	}).Create(context.Background(), runtime.NoConfig{})
	require.NoError(t, err)

	members := readZipMembers(t, filepath.Join(dir, "app.zip"))
	require.Equal(t, map[string]zipMember{
		"bin/app": {body: "binary", mode: 0o755},
	}, members)
}

func TestZipFileCreatesMissingParentDirsWhenOptedIn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "bundle.zip")
	content := "data"
	createDirectory := true

	_, err := (&ZipFile{
		Path:            path,
		Mode:            0o644,
		CreateDirectory: &createDirectory,
		Entries:         &[]ZipEntry{{Name: "data.txt", Content: &content}},
	}).Create(context.Background(), runtime.NoConfig{})
	require.NoError(t, err)

	members := readZipMembers(t, path)
	require.Equal(t, "data", members["data.txt"].body)
}

func TestZipFileFailsWhenParentMissingAndOptOut(t *testing.T) {
	dir := t.TempDir()
	content := "x"

	_, err := (&ZipFile{
		Path:    filepath.Join(dir, "nested", "bundle.zip"),
		Mode:    0o644,
		Entries: &[]ZipEntry{{Name: "x.txt", Content: &content}},
	}).Create(context.Background(), runtime.NoConfig{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no such file or directory")
}

func TestZipFileReadReportsNotFound(t *testing.T) {
	z := &ZipFile{Path: filepath.Join(t.TempDir(), "missing.zip")}
	_, err := z.Read(context.Background(), runtime.NoConfig{}, nil)
	require.True(t, errors.Is(err, runtime.ErrNotFound))
}

func TestZipFileReadFromDisk(t *testing.T) {
	dir := t.TempDir()
	content := "data"
	z := &ZipFile{
		Path:    filepath.Join(dir, "bundle.zip"),
		Mode:    0o644,
		Entries: &[]ZipEntry{{Name: "data.txt", Content: &content}},
	}
	created, err := z.Create(context.Background(), runtime.NoConfig{})
	require.NoError(t, err)

	read, err := z.Read(context.Background(), runtime.NoConfig{}, nil)
	require.NoError(t, err)
	require.Equal(t, created, read)
}

func TestZipFileUpdate(t *testing.T) {
	dir := t.TempDir()
	firstContent := "first"
	z := &ZipFile{
		Path:    filepath.Join(dir, "bundle.zip"),
		Mode:    0o644,
		Entries: &[]ZipEntry{{Name: "data.txt", Content: &firstContent}},
	}
	first, err := z.Create(context.Background(), runtime.NoConfig{})
	require.NoError(t, err)

	secondContent := "second"
	z.Entries = &[]ZipEntry{{Name: "data.txt", Content: &secondContent}}
	second, err := z.Update(context.Background(), runtime.NoConfig{},
		runtime.Prior[ZipFile, *ZipFileOutput]{Outputs: first})
	require.NoError(t, err)
	require.NotEqual(t, first.SHA256, second.SHA256)

	members := readZipMembers(t, filepath.Join(dir, "bundle.zip"))
	require.Equal(t, "second", members["data.txt"].body)
}

func TestZipFileDelete(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bundle.zip")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o644))

	require.NoError(t, (&ZipFile{Path: path}).Delete(context.Background(), runtime.NoConfig{}, nil))
	_, err := os.Stat(path)
	require.True(t, errors.Is(err, os.ErrNotExist))
}

func TestZipFileDeleteAbsentIsNoop(t *testing.T) {
	require.NoError(t, (&ZipFile{Path: filepath.Join(t.TempDir(), "absent.zip")}).
		Delete(context.Background(), runtime.NoConfig{}, nil))
}

func TestZipFileRejectsUnsafeEntryNames(t *testing.T) {
	dir := t.TempDir()
	content := "x"

	_, err := (&ZipFile{
		Path:    filepath.Join(dir, "bundle.zip"),
		Mode:    0o644,
		Entries: &[]ZipEntry{{Name: "../x.txt", Content: &content}},
	}).Create(context.Background(), runtime.NoConfig{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsafe")
}

func TestZipFileRejectsDuplicateEntryNames(t *testing.T) {
	dir := t.TempDir()
	content := "x"

	_, err := (&ZipFile{
		Path: filepath.Join(dir, "bundle.zip"),
		Mode: 0o644,
		Entries: &[]ZipEntry{
			{Name: "x.txt", Content: &content},
			{Name: "./x.txt", Content: &content},
		},
	}).Create(context.Background(), runtime.NoConfig{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate")
}

func TestZipFileRejectsOutputPathInsideSourceDir(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	require.NoError(t, os.MkdirAll(src, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "keep.txt"), []byte("keep"), 0o644))

	_, err := (&ZipFile{
		Path:      filepath.Join(src, "out.zip"),
		Mode:      0o644,
		SourceDir: &src,
	}).Create(context.Background(), runtime.NoConfig{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "inside source-dir")
}

func TestZipFileRequiresExactlyOneSource(t *testing.T) {
	dir := t.TempDir()
	content := "x"
	src := dir

	_, err := (&ZipFile{
		Path:      filepath.Join(dir, "bundle.zip"),
		Mode:      0o644,
		SourceDir: &src,
		Entries:   &[]ZipEntry{{Name: "x.txt", Content: &content}},
	}).Create(context.Background(), runtime.NoConfig{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "exactly one")
}

func TestZipFileEntriesRequireOneInput(t *testing.T) {
	dir := t.TempDir()

	_, err := (&ZipFile{
		Path:    filepath.Join(dir, "bundle.zip"),
		Mode:    0o644,
		Entries: &[]ZipEntry{{Name: "x.txt"}},
	}).Create(context.Background(), runtime.NoConfig{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "exactly one")
}

func TestZipFileDeterministicEntryOrder(t *testing.T) {
	dir := t.TempDir()
	contentA := "a"
	contentB := "b"

	first, err := (&ZipFile{
		Path: filepath.Join(dir, "first.zip"),
		Mode: 0o644,
		Entries: &[]ZipEntry{
			{Name: "b.txt", Content: &contentB},
			{Name: "a.txt", Content: &contentA},
		},
	}).Create(context.Background(), runtime.NoConfig{})
	require.NoError(t, err)

	second, err := (&ZipFile{
		Path: filepath.Join(dir, "second.zip"),
		Mode: 0o644,
		Entries: &[]ZipEntry{
			{Name: "a.txt", Content: &contentA},
			{Name: "b.txt", Content: &contentB},
		},
	}).Create(context.Background(), runtime.NoConfig{})
	require.NoError(t, err)
	require.Equal(t, first.SHA256, second.SHA256)
}

func TestZipFileModifyResourcePlan(t *testing.T) {
	priorContent := "a"
	currentContent := "b"
	prior := ZipFile{Path: "bundle.zip", Entries: &[]ZipEntry{
		{Name: "a.txt", Content: &priorContent},
	}}
	current := ZipFile{Path: "bundle.zip", Entries: &[]ZipEntry{
		{Name: "a.txt", Content: &currentContent},
	}}
	resp := &runtime.ResourcePlanResponse{}

	err := (&ZipFile{}).ModifyResourcePlan(
		runtime.ResourcePlanRequest[ZipFile, *ZipFileOutput, runtime.NoConfig]{
			PriorInputs:   prior,
			CurrentInputs: current,
			HasPriorState: true,
		},
		resp,
	)
	require.NoError(t, err)
	require.Equal(t, map[string]bool{
		"sha256":        true,
		"base64-sha256": true,
		"size":          true,
	}, resp.UnknownOutputs)
}

func TestZipFileReplaceFields(t *testing.T) {
	require.Equal(t, []string{"path"}, (&ZipFile{}).ReplaceFields())
}

func readZipMembers(t *testing.T, path string) map[string]zipMember {
	t.Helper()
	reader, err := zip.OpenReader(path)
	require.NoError(t, err)
	defer func() { require.NoError(t, reader.Close()) }()

	members := make(map[string]zipMember, len(reader.File))
	for _, file := range reader.File {
		rc, err := file.Open()
		require.NoError(t, err)
		body, err := io.ReadAll(rc)
		require.NoError(t, err)
		require.NoError(t, rc.Close())
		members[file.Name] = zipMember{body: string(body), mode: file.Mode().Perm()}
	}
	return members
}
