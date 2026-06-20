package std

import (
	"github.com/cloudboss/unobin/pkg/runtime"

	"github.com/cloudboss/unobin-library-std/internal/exec"
	"github.com/cloudboss/unobin-library-std/internal/fs"
	"github.com/cloudboss/unobin-library-std/internal/net"
)

// Library returns the registration record for the std library: the
// actions and resources that do I/O, the counterpart to the pure
// functions the language provides under @core. A stack reaches them
// under its chosen alias, std by convention:
// actions: { std: { command: { ... } } } and
// resources: { std: { file: { ... } } }.
func Library() *runtime.Library {
	return &runtime.Library{
		Name:        "std",
		Description: "Standard actions and resources",
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
			"fs-file": runtime.MakeResource[fs.File, *fs.FileOutput, runtime.NoConfig](),
		},
	}
}
