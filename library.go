package std

import (
	"github.com/cloudboss/unobin/pkg/runtime"

	"github.com/cloudboss/unobin-library-std/internal/archive"
	"github.com/cloudboss/unobin-library-std/internal/exec"
	"github.com/cloudboss/unobin-library-std/internal/fs"
	"github.com/cloudboss/unobin-library-std/internal/net"
	"github.com/cloudboss/unobin-library-std/internal/random"
)

// Library returns the registration record for the std library: the
// actions and resources that do I/O, the counterpart to the pure
// functions the language provides under @core. A stack reaches them
// under its chosen alias, std by convention:
// actions: { std: { command: { ... } } } and
// resources: { std: { file: { ... } } }.
func Library() *runtime.Library {
	return &runtime.Library{
		Name:          "std",
		Description:   "Standard actions and resources",
		Compatibility: runtime.LibraryCompatibility{RequiredAPI: "1.0"},
		Actions: map[string]runtime.ActionRegistration{
			"exec-command": runtime.MakeAction[
				exec.CommandAction,
				*exec.CommandActionOutput,
				runtime.NoConfig,
			](),
			"exec-script": runtime.MakeAction[
				exec.ScriptAction,
				*exec.ScriptActionOutput,
				runtime.NoConfig,
			](),
			"net-http": runtime.MakeAction[
				net.HTTPAction,
				*net.HTTPActionOutput,
				runtime.NoConfig,
			](),
			"exec-wait-for": runtime.MakeAction[
				exec.WaitForAction,
				*exec.WaitForActionOutput,
				runtime.NoConfig,
			](),
		},
		Resources: map[string]runtime.ResourceRegistration{
			"archive-zipfile": runtime.MakeResource[
				archive.ZipFile,
				*archive.ZipFileOutput,
				runtime.NoConfig,
			](archive.ZipFileDefinition()),
			"fs-file": runtime.MakeResource[fs.File, *fs.FileOutput, runtime.NoConfig](
				fs.FileDefinition(),
			),
			"random-id": runtime.MakeResource[
				random.ID,
				*random.IDOutput,
				runtime.NoConfig,
			](random.IDDefinition()),
		},
	}
}
