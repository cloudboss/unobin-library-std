package exec

import (
	"context"
	"testing"

	"github.com/cloudboss/unobin/pkg/runtime"
	"github.com/stretchr/testify/require"
)

func runScript(t *testing.T, a *ScriptAction) *CommandActionOutput {
	t.Helper()
	res, err := a.Run(context.Background(), runtime.NoConfig{})
	require.NoError(t, err)
	require.NotNil(t, res)
	return res
}

func TestScriptRunsThroughShell(t *testing.T) {
	cr := runScript(t, &ScriptAction{Shell: "sh", Script: "echo hello"})
	require.Equal(t, "hello\n", cr.Stdout)
	require.Equal(t, 0, cr.ExitCode)
}

func TestScriptMultiline(t *testing.T) {
	cr := runScript(t, &ScriptAction{
		Shell:  "sh",
		Script: "echo one\necho two\necho three\n",
	})
	require.Equal(t, "one\ntwo\nthree\n", cr.Stdout)
}

func TestScriptExpandsEnvironment(t *testing.T) {
	cr := runScript(t, &ScriptAction{
		Shell:       "sh",
		Script:      "echo \"$UNOBIN_TEST_KEY\"",
		Environment: map[string]string{"UNOBIN_TEST_KEY": "abc123"},
	})
	require.Equal(t, "abc123\n", cr.Stdout)
}

func TestScriptCustomShell(t *testing.T) {
	cr := runScript(t, &ScriptAction{
		Shell:  "bash",
		Script: "echo $BASH_VERSION",
	})
	require.NotEmpty(t, cr.Stdout)
}

func TestScriptReportsExitCode(t *testing.T) {
	cr := runScript(t, &ScriptAction{Shell: "sh", Script: "exit 9"})
	require.Equal(t, 9, cr.ExitCode)
}

func TestScriptRequiresBody(t *testing.T) {
	_, err := (&ScriptAction{}).Run(context.Background(), runtime.NoConfig{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "script is required")
}
