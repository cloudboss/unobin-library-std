package random

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"github.com/cloudboss/unobin/pkg/constraint"
	"github.com/cloudboss/unobin/pkg/runtime"
)

// ID generates cryptographically random identifiers.
type ID struct {
	// ByteLength is the number of random bytes to generate and must be at least one.
	ByteLength int64
	// Keepers requests a new identifier when any map value changes.
	Keepers *map[string]string
	// Prefix is prepended verbatim to every encoded output except ID.
	Prefix *string
}

// IDOutput provides the identifier in each supported encoding.
type IDOutput struct {
	// ID is the unpadded URL-safe Base64 encoding without Prefix.
	ID string
	// B64URL is Prefix followed by the unpadded URL-safe Base64 encoding.
	B64URL string
	// B64Std is Prefix followed by the padded standard Base64 encoding.
	B64Std string
	// Dec is Prefix followed by the unsigned big-endian decimal encoding.
	Dec string
	// Hex is Prefix followed by the lowercase hexadecimal encoding.
	Hex string
}

func IDDefinition() runtime.ResourceDefinition[ID, *IDOutput, runtime.NoConfig] {
	byteLength := runtime.InputField(func(input *ID) *int64 { return &input.ByteLength })
	keepers := runtime.InputField(func(input *ID) **map[string]string { return &input.Keepers })
	prefix := runtime.InputField(func(input *ID) **string { return &input.Prefix })
	return runtime.ResourceDefinition[ID, *IDOutput, runtime.NoConfig]{
		SchemaVersion: 1,
		Replace: runtime.Replacement[ID, *IDOutput, runtime.NoConfig]{
			Fields: []runtime.AnyInputField[ID]{byteLength, keepers, prefix},
		},
		StableID: func(_ ID, output *IDOutput) (string, error) {
			if output == nil || output.ID == "" {
				return "", errors.New("random-id: output is missing ID")
			}
			return output.ID, nil
		},
	}
}

func (r ID) Constraints() []constraint.Constraint {
	return []constraint.Constraint{
		constraint.Must(constraint.AtLeast(r.ByteLength, 1)).
			Message("byte-length must be at least 1"),
	}
}

func (r *ID) Create(_ context.Context, _ runtime.NoConfig) (*IDOutput, error) {
	if r.ByteLength < 1 {
		return nil, errors.New("random-id: byte-length must be at least 1")
	}
	byteLength := int(r.ByteLength)
	if int64(byteLength) != r.ByteLength {
		return nil, errors.New("random-id: byte-length is too large")
	}

	bytes := make([]byte, byteLength)
	if _, err := rand.Read(bytes); err != nil {
		return nil, fmt.Errorf("random-id: generate bytes: %w", err)
	}
	prefix := ""
	if r.Prefix != nil {
		prefix = *r.Prefix
	}
	return encodeID(bytes, prefix), nil
}

func (r *ID) Read(
	_ context.Context,
	_ runtime.NoConfig,
	prior runtime.Prior[ID, *IDOutput, runtime.NoConfig],
) (*IDOutput, error) {
	if prior.Outputs == nil {
		return nil, runtime.ErrNotFound
	}
	return prior.Outputs, nil
}

func (r *ID) Update(
	_ context.Context,
	_ runtime.NoConfig,
	prior runtime.Prior[ID, *IDOutput, runtime.NoConfig],
) (*IDOutput, error) {
	if prior.Outputs == nil {
		return nil, runtime.ErrNotFound
	}
	return prior.Outputs, nil
}

func (r *ID) Delete(
	_ context.Context,
	_ runtime.NoConfig,
	_ runtime.Prior[ID, *IDOutput, runtime.NoConfig],
) error {
	return nil
}

func encodeID(bytes []byte, prefix string) *IDOutput {
	id := base64.RawURLEncoding.EncodeToString(bytes)
	decimal := new(big.Int).SetBytes(bytes).String()
	return &IDOutput{
		ID:     id,
		B64URL: prefix + id,
		B64Std: prefix + base64.StdEncoding.EncodeToString(bytes),
		Dec:    prefix + decimal,
		Hex:    prefix + hex.EncodeToString(bytes),
	}
}
