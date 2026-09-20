package random

import (
	"context"
	"encoding/base64"
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudboss/unobin/pkg/runtime"
)

func TestEncodeID(t *testing.T) {
	tests := []struct {
		name   string
		bytes  []byte
		prefix string
		want   *IDOutput
	}{
		{
			name:   "leading zero",
			bytes:  []byte{0x00, 0x01, 0xfe, 0xff},
			prefix: "web-",
			want: &IDOutput{
				ID:     "AAH-_w",
				B64URL: "web-AAH-_w",
				B64Std: "web-AAH+/w==",
				Dec:    "web-130815",
				Hex:    "web-0001feff",
			},
		},
		{
			name:  "zero",
			bytes: []byte{0x00},
			want: &IDOutput{
				ID:     "AA",
				B64URL: "AA",
				B64Std: "AA==",
				Dec:    "0",
				Hex:    "00",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, encodeID(tt.bytes, tt.prefix))
		})
	}
}

func TestIDCreate(t *testing.T) {
	prefix := "web-"
	out, err := (&ID{ByteLength: 16, Prefix: &prefix}).Create(
		context.Background(), runtime.NoConfig{},
	)
	require.NoError(t, err)

	raw, err := base64.RawURLEncoding.DecodeString(out.ID)
	require.NoError(t, err)
	require.Len(t, raw, 16)
	require.Equal(t, encodeID(raw, prefix), out)
}

func TestIDCreateRejectsInvalidByteLength(t *testing.T) {
	for _, byteLength := range []int64{-1, 0} {
		t.Run(strconv.FormatInt(byteLength, 10), func(t *testing.T) {
			_, err := (&ID{ByteLength: byteLength}).Create(
				context.Background(), runtime.NoConfig{},
			)
			require.Error(t, err)
			require.Contains(t, err.Error(), "byte-length must be at least 1")
		})
	}
}

func TestIDReadReturnsPriorOutput(t *testing.T) {
	prior := &IDOutput{ID: "fixed"}
	out, err := (&ID{}).Read(context.Background(), runtime.NoConfig{},
		runtime.Prior[ID, *IDOutput, runtime.NoConfig]{Outputs: prior})
	require.NoError(t, err)
	require.Same(t, prior, out)
}

func TestIDReadReportsNotFoundWithoutPriorOutput(t *testing.T) {
	out, err := (&ID{}).Read(context.Background(), runtime.NoConfig{},
		runtime.Prior[ID, *IDOutput, runtime.NoConfig]{})
	require.Nil(t, out)
	require.True(t, errors.Is(err, runtime.ErrNotFound))
}

func TestIDUpdatePreservesPriorOutput(t *testing.T) {
	prior := &IDOutput{ID: "fixed"}
	out, err := (&ID{}).Update(
		context.Background(),
		runtime.NoConfig{},
		runtime.Prior[ID, *IDOutput, runtime.NoConfig]{Outputs: prior},
	)
	require.NoError(t, err)
	require.Same(t, prior, out)
}

func TestIDDeleteIsNoop(t *testing.T) {
	require.NoError(t, (&ID{}).Delete(
		context.Background(), runtime.NoConfig{},
		runtime.Prior[ID, *IDOutput, runtime.NoConfig]{Outputs: &IDOutput{ID: "fixed"}},
	))
}

func TestIDDefinition(t *testing.T) {
	definition := IDDefinition()
	require.Equal(t, 1, definition.SchemaVersion)
	require.NotNil(t, definition.Validate)
	require.Len(t, definition.Replace.Fields, 3)
	require.NotNil(t, definition.StableID)
	require.NotPanics(t, func() {
		runtime.MakeResource[ID, *IDOutput, runtime.NoConfig](definition)
	})

	id, err := definition.StableID(ID{}, &IDOutput{ID: "fixed"})
	require.NoError(t, err)
	require.Equal(t, "fixed", id)
	for _, output := range []*IDOutput{nil, {}} {
		_, err := definition.StableID(ID{}, output)
		require.ErrorContains(t, err, "missing ID")
	}
	err = definition.Validate(context.Background(), ID{}, runtime.NoConfig{})
	require.ErrorContains(t, err, "byte-length must be at least 1")
}
