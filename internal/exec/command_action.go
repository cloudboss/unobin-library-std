package exec

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cloudboss/unobin/pkg/runtime"
)

// CommandAction execs a single process and captures its output.
type CommandAction struct {
	Argv        []string
	Environment *map[string]string
	WorkingDir  *string
}

// CommandActionOutput holds the captured output of a command run. Run returns
// an error when the process fails to start or the context is canceled.
type CommandActionOutput struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}

// Run execs argv[0] with argv[1:] as arguments. Environment is merged
// with the parent, with user-supplied variables taking precedence.
func (a *CommandAction) Run(ctx context.Context, _ runtime.NoConfig) (*CommandActionOutput, error) {
	if len(a.Argv) == 0 {
		return nil, errors.New("argv is required")
	}
	if a.Argv[0] == "" {
		return nil, fmt.Errorf("argv[0] is empty")
	}
	return runProcess(ctx, processSpec{
		Argv:        a.Argv,
		Environment: optionalMapValue(a.Environment),
		WorkingDir:  optionalStringValue(a.WorkingDir),
	})
}
