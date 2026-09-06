package std

import (
	"maps"
	"slices"
	"testing"

	"github.com/cloudboss/unobin/pkg/runtime"
	"github.com/stretchr/testify/require"

	"github.com/cloudboss/unobin-library-std/internal/exec"
	"github.com/cloudboss/unobin-library-std/internal/net"
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

	require.Equal(t, []string{"archive-zipfile", "fs-file", "random-id"},
		slices.Sorted(maps.Keys(lib.Resources)))
	require.Equal(t, []string{"exec-command", "exec-script", "exec-wait-for", "net-http"},
		slices.Sorted(maps.Keys(lib.Actions)))
	catalog, err := runtime.NewLibraryCatalog([]runtime.LibraryRegistration{
		{LibraryPath: libraryPath, New: Library},
	})
	require.NoError(t, err)
	libraries, err := catalog.Libraries(map[string]string{"std": libraryPath, "other": libraryPath})
	require.NoError(t, err)
	require.Same(t, libraries["std"], libraries["other"])
	require.Equal(t, libraryPath, libraries["std"].LibraryPath)
}
