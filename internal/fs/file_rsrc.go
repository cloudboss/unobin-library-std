package fs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	iofs "io/fs"
	"os"
	"path/filepath"

	"github.com/cloudboss/unobin/pkg/defaults"
	ufs "github.com/cloudboss/unobin/pkg/fs"
	"github.com/cloudboss/unobin/pkg/runtime"
)

// File writes a regular file to the local filesystem. The file's path
// is part of the input set; changing the path replaces the resource
// (the prior file is deleted and a new one is written at the new path).
// CreateDirectory opts the resource into creating any missing parent
// directories of Path. Without it, a missing parent is an error so
// callers do not accidentally write outside an expected tree.
type File struct {
	Path            string
	Content         string
	Mode            int64
	CreateDirectory *bool
}

// FileOutput is what gets stored in state after Create / Update. It
// holds only what writing the file computes; path is an input and is
// readable as one, so it is not copied here.
type FileOutput struct {
	SHA256 string
	Size   int64
}

func FileDefinition() runtime.ResourceDefinition[File, *FileOutput, runtime.NoConfig] {
	path := runtime.InputField(func(input *File) *string { return &input.Path })
	return runtime.ResourceDefinition[File, *FileOutput, runtime.NoConfig]{
		SchemaVersion: 1,
		Validate: func(_ context.Context, input File, _ runtime.NoConfig) error {
			return input.validate()
		},
		Replace: runtime.Replacement[File, *FileOutput, runtime.NoConfig]{
			Fields: []runtime.AnyInputField[File]{path},
		},
	}
}

// Defaults declares the inputs a body may leave out: mode defaults to 0o644.
func (f File) Defaults() []defaults.Default {
	return []defaults.Default{
		defaults.Value(f.Mode, 0o644),
	}
}

func (f *File) Create(_ context.Context, _ runtime.NoConfig) (*FileOutput, error) {
	return f.write()
}

func (f *File) Read(
	_ context.Context,
	_ runtime.NoConfig,
	prior runtime.Prior[File, *FileOutput, runtime.NoConfig],
) (*FileOutput, error) {
	info, err := os.Stat(prior.Inputs.Path)
	if err != nil {
		if errors.Is(err, iofs.ErrNotExist) {
			return nil, runtime.ErrNotFound
		}
		return nil, err
	}
	body, err := os.ReadFile(prior.Inputs.Path)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(body)
	return &FileOutput{
		SHA256: hex.EncodeToString(sum[:]),
		Size:   info.Size(),
	}, nil
}

func (f *File) Update(
	_ context.Context,
	_ runtime.NoConfig,
	_ runtime.Prior[File, *FileOutput, runtime.NoConfig],
) (*FileOutput, error) {
	return f.write()
}

func (f *File) Delete(
	_ context.Context,
	_ runtime.NoConfig,
	prior runtime.Prior[File, *FileOutput, runtime.NoConfig],
) error {
	err := os.Remove(prior.Inputs.Path)
	if err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return err
	}
	return nil
}

func (f *File) write() (*FileOutput, error) {
	if err := f.validate(); err != nil {
		return nil, err
	}
	mode := os.FileMode(f.Mode)
	if f.CreateDirectory != nil && *f.CreateDirectory {
		if err := os.MkdirAll(filepath.Dir(f.Path), 0o755); err != nil {
			return nil, err
		}
	}
	body := []byte(f.Content)
	if err := ufs.WriteFileAtomic(f.Path, body, mode); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(body)
	return &FileOutput{
		SHA256: hex.EncodeToString(sum[:]),
		Size:   int64(len(body)),
	}, nil
}

func (f *File) validate() error {
	if f.Path == "" {
		return errors.New("local.file: path is required")
	}
	return nil
}
