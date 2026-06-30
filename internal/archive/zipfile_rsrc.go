package archive

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	iofs "io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cloudboss/unobin/pkg/constraint"
	"github.com/cloudboss/unobin/pkg/defaults"
	"github.com/cloudboss/unobin/pkg/runtime"
)

const defaultEntryMode os.FileMode = 0o644

var zipModTime = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)

// ZipFile writes a zip archive to the local filesystem.
type ZipFile struct {
	Path            string
	Mode            int64
	CreateDirectory *bool
	SourceDir       *string
	SourceFile      *string
	EntryName       *string
	Entries         *[]ZipEntry
	Excludes        *[]string
	EntryMode       *int64
}

// ZipEntry is one member of a zip archive.
type ZipEntry struct {
	Name       string
	Content    *string
	SourceFile *string
	Mode       *int64
}

// ZipFileOutput records the digest and byte count of the zip archive.
type ZipFileOutput struct {
	SHA256       string
	Base64SHA256 string
	Size         int64
}

type zipItem struct {
	name     string
	filePath string
	content  *string
	mode     os.FileMode
}

type countWriter struct {
	n int64
}

func (w *countWriter) Write(p []byte) (int, error) {
	w.n += int64(len(p))
	return len(p), nil
}

func (z *ZipFile) SchemaVersion() int      { return 1 }
func (z *ZipFile) ReplaceFields() []string { return []string{"path"} }

func (z ZipFile) Defaults() []defaults.Default {
	return []defaults.Default{
		defaults.Value(z.Mode, 0o644),
	}
}

func (z ZipFile) Constraints() []constraint.Constraint {
	return []constraint.Constraint{
		constraint.ExactlyOneOf(z.SourceDir, z.SourceFile, z.Entries),
		constraint.ForbiddenWith(z.EntryName, z.SourceDir, z.Entries),
		constraint.ForbiddenWith(z.Excludes, z.SourceFile, z.Entries),
		constraint.ForEach(z.Entries, func(e ZipEntry) []constraint.Constraint {
			return []constraint.Constraint{
				constraint.ExactlyOneOf(e.Content, e.SourceFile),
			}
		}),
	}
}

func (z *ZipFile) Create(_ context.Context, _ runtime.NoConfig) (*ZipFileOutput, error) {
	return z.write()
}

func (z *ZipFile) Read(
	_ context.Context, _ runtime.NoConfig, _ *ZipFileOutput,
) (*ZipFileOutput, error) {
	return readZipFileOutput(z.Path)
}

func (z *ZipFile) Update(
	_ context.Context, _ runtime.NoConfig, _ runtime.Prior[ZipFile, *ZipFileOutput],
) (*ZipFileOutput, error) {
	return z.write()
}

func (z *ZipFile) Delete(_ context.Context, _ runtime.NoConfig, _ *ZipFileOutput) error {
	err := os.Remove(z.Path)
	if err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return err
	}
	return nil
}

func (z *ZipFile) ModifyResourcePlan(
	req runtime.ResourcePlanRequest[ZipFile, *ZipFileOutput, runtime.NoConfig],
	resp *runtime.ResourcePlanResponse,
) error {
	if req.HasPriorState && runtime.Changed(req.PriorInputs, req.CurrentInputs) {
		resp.MarkOutputUnknown("sha256", "base64-sha256", "size")
	}
	return nil
}

func (z *ZipFile) write() (*ZipFileOutput, error) {
	if z.Path == "" {
		return nil, errors.New("archive-zipfile: path is required")
	}
	if z.CreateDirectory != nil && *z.CreateDirectory {
		if err := os.MkdirAll(filepath.Dir(z.Path), 0o755); err != nil {
			return nil, err
		}
	}
	items, err := z.collectItems()
	if err != nil {
		return nil, err
	}
	return writeZipAtomic(z.Path, os.FileMode(z.Mode), items)
}

func (z *ZipFile) collectItems() ([]zipItem, error) {
	if err := z.validateInputSet(); err != nil {
		return nil, err
	}

	var (
		items []zipItem
		err   error
	)
	switch {
	case z.SourceDir != nil:
		items, err = z.collectDirItems(*z.SourceDir)
	case z.SourceFile != nil:
		items, err = z.collectFileItem(*z.SourceFile)
	case z.Entries != nil:
		items, err = z.collectEntryItems(*z.Entries)
	}
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, errors.New("archive-zipfile: zip would be empty")
	}
	if err := rejectDuplicateNames(items); err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool { return items[i].name < items[j].name })
	return items, nil
}

func (z *ZipFile) validateInputSet() error {
	count := 0
	if z.SourceDir != nil {
		count++
	}
	if z.SourceFile != nil {
		count++
	}
	if z.Entries != nil {
		count++
	}
	if count != 1 {
		return errors.New(
			"archive-zipfile: exactly one of source-dir, source-file, or entries is required",
		)
	}
	if z.Excludes != nil && z.SourceDir == nil {
		return errors.New("archive-zipfile: excludes requires source-dir")
	}
	if z.EntryName != nil && z.SourceFile == nil {
		return errors.New("archive-zipfile: entry-name requires source-file")
	}
	return nil
}

func (z *ZipFile) collectDirItems(dir string) ([]zipItem, error) {
	if dir == "" {
		return nil, errors.New("archive-zipfile: source-dir is required")
	}
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("archive-zipfile: stat source-dir: %w", err)
	}
	if !info.IsDir() {
		return nil, errors.New("archive-zipfile: source-dir must be a directory")
	}
	excludes := optionalStrings(z.Excludes)
	if err := z.rejectOutputInsideSourceDir(dir, excludes); err != nil {
		return nil, err
	}

	var items []zipItem
	err = filepath.WalkDir(dir, func(filePath string, entry iofs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("archive-zipfile: walk source-dir: %w", err)
		}
		if filePath == dir {
			return nil
		}
		rel, err := filepath.Rel(dir, filePath)
		if err != nil {
			return fmt.Errorf("archive-zipfile: relative path: %w", err)
		}
		name, err := cleanZipName(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		matched, err := excluded(name, excludes)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if matched {
				return filepath.SkipDir
			}
			return nil
		}
		if matched {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("archive-zipfile: stat source file: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("archive-zipfile: symlink %q is not supported", filePath)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("archive-zipfile: %q is not a regular file", filePath)
		}
		items = append(items, zipItem{
			name:     name,
			filePath: filePath,
			mode:     z.memberMode(nil, info.Mode().Perm()),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (z *ZipFile) collectFileItem(filePath string) ([]zipItem, error) {
	info, err := regularFileInfo(filePath, "source-file")
	if err != nil {
		return nil, err
	}
	name := filepath.Base(filePath)
	if z.EntryName != nil {
		name = *z.EntryName
	}
	name, err = cleanZipName(filepath.ToSlash(name))
	if err != nil {
		return nil, err
	}
	return []zipItem{{
		name:     name,
		filePath: filePath,
		mode:     z.memberMode(nil, info.Mode().Perm()),
	}}, nil
}

func (z *ZipFile) collectEntryItems(entries []ZipEntry) ([]zipItem, error) {
	items := make([]zipItem, 0, len(entries))
	for i, entry := range entries {
		name, err := cleanZipName(entry.Name)
		if err != nil {
			return nil, fmt.Errorf("archive-zipfile: entries[%d]: %w", i, err)
		}
		count := 0
		if entry.Content != nil {
			count++
		}
		if entry.SourceFile != nil {
			count++
		}
		if count != 1 {
			return nil, fmt.Errorf(
				"archive-zipfile: entries[%d] requires exactly one of content or source-file",
				i,
			)
		}
		if entry.Content != nil {
			content := *entry.Content
			items = append(items, zipItem{
				name:    name,
				content: &content,
				mode:    z.memberMode(entry.Mode, defaultEntryMode),
			})
			continue
		}

		info, err := regularFileInfo(*entry.SourceFile, "entries.source-file")
		if err != nil {
			return nil, err
		}
		items = append(items, zipItem{
			name:     name,
			filePath: *entry.SourceFile,
			mode:     z.memberMode(entry.Mode, info.Mode().Perm()),
		})
	}
	return items, nil
}

func (z *ZipFile) memberMode(entryMode *int64, fallback os.FileMode) os.FileMode {
	mode := fallback
	if z.EntryMode != nil {
		mode = os.FileMode(*z.EntryMode)
	}
	if entryMode != nil {
		mode = os.FileMode(*entryMode)
	}
	return mode.Perm()
}

func (z *ZipFile) rejectOutputInsideSourceDir(sourceDir string, excludes []string) error {
	absSource, err := filepath.Abs(sourceDir)
	if err != nil {
		return err
	}
	absOutput, err := filepath.Abs(z.Path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(absSource, absOutput)
	if err != nil {
		return nil
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil
	}
	name := filepath.ToSlash(rel)
	matched, err := excluded(name, excludes)
	if err != nil {
		return err
	}
	if matched {
		return nil
	}
	return fmt.Errorf(
		"archive-zipfile: path %q is inside source-dir %q; add an exclude for %q",
		z.Path,
		sourceDir,
		name,
	)
}

func regularFileInfo(filePath, field string) (os.FileInfo, error) {
	if filePath == "" {
		return nil, fmt.Errorf("archive-zipfile: %s is required", field)
	}
	info, err := os.Lstat(filePath)
	if err != nil {
		return nil, fmt.Errorf("archive-zipfile: stat %s: %w", field, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("archive-zipfile: %s %q is a symlink", field, filePath)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("archive-zipfile: %s %q is not a regular file", field, filePath)
	}
	return info, nil
}

func cleanZipName(name string) (string, error) {
	if name == "" {
		return "", errors.New("entry name is required")
	}
	if strings.ContainsRune(name, '\x00') || strings.Contains(name, "\\") {
		return "", fmt.Errorf("unsafe entry name %q", name)
	}
	if hasWindowsVolume(name) || hasParentSegment(name) {
		return "", fmt.Errorf("unsafe entry name %q", name)
	}
	cleaned := path.Clean(name)
	if cleaned == "." || cleaned == ".." || path.IsAbs(cleaned) || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("unsafe entry name %q", name)
	}
	return cleaned, nil
}

func hasWindowsVolume(name string) bool {
	return len(name) >= 2 && name[1] == ':'
}

func hasParentSegment(name string) bool {
	for part := range strings.SplitSeq(name, "/") {
		if part == ".." {
			return true
		}
	}
	return false
}

func rejectDuplicateNames(items []zipItem) error {
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		if seen[item.name] {
			return fmt.Errorf("archive-zipfile: duplicate entry name %q", item.name)
		}
		seen[item.name] = true
	}
	return nil
}

func optionalStrings(values *[]string) []string {
	if values == nil {
		return nil
	}
	return *values
}

func excluded(name string, patterns []string) (bool, error) {
	for _, pattern := range patterns {
		pattern = filepath.ToSlash(pattern)
		if pattern == "" {
			continue
		}
		if path.IsAbs(pattern) || hasWindowsVolume(pattern) || hasParentSegment(pattern) {
			return false, fmt.Errorf("archive-zipfile: exclude pattern %q must be relative", pattern)
		}
		pattern = path.Clean(pattern)
		if pattern == "." {
			continue
		}
		matched, err := matchGlob(pattern, name)
		if err != nil {
			return false, fmt.Errorf("archive-zipfile: exclude pattern %q: %w", pattern, err)
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

func matchGlob(pattern, name string) (bool, error) {
	return matchGlobParts(strings.Split(pattern, "/"), strings.Split(name, "/"))
}

func matchGlobParts(pattern, name []string) (bool, error) {
	if len(pattern) == 0 {
		return len(name) == 0, nil
	}
	if pattern[0] == "**" {
		for i := 0; i <= len(name); i++ {
			matched, err := matchGlobParts(pattern[1:], name[i:])
			if err != nil || matched {
				return matched, err
			}
		}
		return false, nil
	}
	if len(name) == 0 {
		return false, nil
	}
	matched, err := path.Match(pattern[0], name[0])
	if err != nil || !matched {
		return matched, err
	}
	return matchGlobParts(pattern[1:], name[1:])
}

func writeZipAtomic(outputPath string, mode os.FileMode, items []zipItem) (*ZipFileOutput, error) {
	base := filepath.Base(outputPath)
	if base == "" || base == "." || base == string(filepath.Separator) {
		return nil, errors.New("archive-zipfile: path is invalid")
	}

	tmp, err := os.CreateTemp(filepath.Dir(outputPath), "."+base+".*.tmp")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	closed := false
	remove := true
	defer func() {
		if !closed {
			_ = tmp.Close()
		}
		if remove {
			_ = os.Remove(tmpPath)
		}
	}()

	sum := sha256.New()
	counter := &countWriter{}
	zw := zip.NewWriter(io.MultiWriter(tmp, sum, counter))
	for _, item := range items {
		if err := addZipItem(zw, item); err != nil {
			_ = zw.Close()
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	if err := tmp.Chmod(mode); err != nil {
		return nil, err
	}
	if err := tmp.Sync(); err != nil {
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	closed = true
	if err := os.Rename(tmpPath, outputPath); err != nil {
		return nil, err
	}
	remove = false
	syncDir(filepath.Dir(outputPath))

	bytes := sum.Sum(nil)
	return &ZipFileOutput{
		SHA256:       hex.EncodeToString(bytes),
		Base64SHA256: base64.StdEncoding.EncodeToString(bytes),
		Size:         counter.n,
	}, nil
}

func addZipItem(zw *zip.Writer, item zipItem) error {
	header := &zip.FileHeader{Name: item.name, Method: zip.Deflate}
	header.SetMode(item.mode)
	header.Modified = zipModTime

	writer, err := zw.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("archive-zipfile: create entry %q: %w", item.name, err)
	}
	if item.content != nil {
		if _, err := io.WriteString(writer, *item.content); err != nil {
			return fmt.Errorf("archive-zipfile: write entry %q: %w", item.name, err)
		}
		return nil
	}

	file, err := os.Open(item.filePath)
	if err != nil {
		return fmt.Errorf("archive-zipfile: open source file %q: %w", item.filePath, err)
	}
	defer func() { _ = file.Close() }()
	if _, err := io.Copy(writer, file); err != nil {
		return fmt.Errorf("archive-zipfile: write entry %q: %w", item.name, err)
	}
	return nil
}

func readZipFileOutput(outputPath string) (*ZipFileOutput, error) {
	file, err := os.Open(outputPath)
	if err != nil {
		if errors.Is(err, iofs.ErrNotExist) {
			return nil, runtime.ErrNotFound
		}
		return nil, err
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("archive-zipfile: path %q is a directory", outputPath)
	}

	sum := sha256.New()
	if _, err := io.Copy(sum, file); err != nil {
		return nil, err
	}
	bytes := sum.Sum(nil)
	return &ZipFileOutput{
		SHA256:       hex.EncodeToString(bytes),
		Base64SHA256: base64.StdEncoding.EncodeToString(bytes),
		Size:         info.Size(),
	}, nil
}

func syncDir(dir string) {
	parent, err := os.Open(dir)
	if err != nil {
		return
	}
	defer func() { _ = parent.Close() }()
	_ = parent.Sync()
}
