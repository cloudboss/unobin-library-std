package exec

import (
	"context"
	"errors"

	"github.com/cloudboss/unobin/pkg/defaults"
	"github.com/cloudboss/unobin/pkg/runtime"
)

// ScriptAction runs a shell script via `<shell> -c <script>`. Shell defaults
// to `sh`; set it to `bash`, `python3`, or any other interpreter that
// accepts `-c`.
type ScriptAction struct {
	Script      string
	Shell       string
	Environment *map[string]string
	WorkingDir  *string
}

// Defaults declares the inputs a body may leave out: the shell
// defaults to sh.
func (a ScriptAction) Defaults() []defaults.Default {
	return []defaults.Default{
		defaults.Value(a.Shell, "sh"),
	}
}

// ScriptActionOutput is the captured output of a script run. It is the
// same form as the command action's output because both reduce to a
// process exec; the alias keeps the convention that every action type
// has a sibling type named `<GoName>Output`.
type ScriptActionOutput = CommandActionOutput

// Run invokes the configured shell with the script. Output mirrors
// what CommandAction returns.
func (a *ScriptAction) Run(ctx context.Context, _ runtime.NoConfig) (*ScriptActionOutput, error) {
	if a.Script == "" {
		return nil, errors.New("script is required")
	}
	return runProcess(ctx, processSpec{
		Argv:        []string{a.Shell, "-c", a.Script},
		Environment: optionalMapValue(a.Environment),
		WorkingDir:  optionalStringValue(a.WorkingDir),
	})
}
