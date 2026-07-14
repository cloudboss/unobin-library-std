package std

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudboss/unobin-library-std/internal/archive"
	"github.com/cloudboss/unobin-library-std/internal/exec"
	"github.com/cloudboss/unobin-library-std/internal/fs"
	"github.com/cloudboss/unobin-library-std/internal/net"
	"github.com/cloudboss/unobin-library-std/internal/random"
)

func TestLibraryRegistrations(t *testing.T) {
	lib := Library()
	require.Equal(t, "std", lib.Name)

	command, ok := lib.Actions["exec-command"]
	require.True(t, ok)
	_, ok = command.NewReceiver().(*exec.CommandAction)
	require.True(t, ok)

	script, ok := lib.Actions["exec-script"]
	require.True(t, ok)
	_, ok = script.NewReceiver().(*exec.ScriptAction)
	require.True(t, ok)

	httpAction, ok := lib.Actions["net-http"]
	require.True(t, ok)
	_, ok = httpAction.NewReceiver().(*net.HTTPAction)
	require.True(t, ok)

	waitFor, ok := lib.Actions["exec-wait-for"]
	require.True(t, ok)
	_, ok = waitFor.NewReceiver().(*exec.WaitForAction)
	require.True(t, ok)

	zipFileRes, ok := lib.Resources["archive-zipfile"]
	require.True(t, ok)
	require.Equal(t, 1, zipFileRes.SchemaVersion())
	_, ok = zipFileRes.NewReceiver().(*archive.ZipFile)
	require.True(t, ok)

	fileRes, ok := lib.Resources["fs-file"]
	require.True(t, ok)
	require.Equal(t, 1, fileRes.SchemaVersion())
	_, ok = fileRes.NewReceiver().(*fs.File)
	require.True(t, ok)

	randomIDRes, ok := lib.Resources["random-id"]
	require.True(t, ok)
	require.Equal(t, 1, randomIDRes.SchemaVersion())
	_, ok = randomIDRes.NewReceiver().(*random.ID)
	require.True(t, ok)
}
