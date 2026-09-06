package std

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudboss/unobin/pkg/encrypters"
	"github.com/cloudboss/unobin/pkg/runtime"
	"github.com/cloudboss/unobin/pkg/sdk/encrypt"
	"github.com/cloudboss/unobin/pkg/sdk/state"
	"github.com/cloudboss/unobin/pkg/state/local"
	"github.com/stretchr/testify/require"
)

func TestObsoleteArtifactsRejectBeforeProviderMutation(t *testing.T) {
	body, err := json.Marshal(map[string]any{"format-version": 1})
	require.NoError(t, err)
	for _, artifact := range []string{"state", "plan"} {
		t.Run(artifact, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "managed.txt")
			executor := consumerExecutor(t, "file", t.TempDir(),
				filesystemInputs("file", path, "original"))
			_, prior := applySavedPlan(t, executor, savedPlan(t, executor))
			executor.Inputs["content"] = "changed"
			if artifact == "state" {
				store := executor.Store.(*local.Store)
				sealed, err := state.Seal(body, state.PayloadTypeState, encrypters.Noop{})
				require.NoError(t, err)
				oldPath := filepath.Join(store.Root, store.Factory, store.Stack(),
					"snapshots", "obsolete.json.enc")
				require.NoError(t, os.WriteFile(oldPath, sealed, 0o600))
				require.NoError(t, store.SetCurrent("obsolete"))
				plan, err := executor.PlanV2(t.Context())
				require.ErrorContains(t, err, "obsolete alpha format")
				require.Nil(t, plan)
				revision, err := store.CurrentRev()
				require.NoError(t, err)
				require.Equal(t, "obsolete", revision)
			} else {
				sealed, err := state.Seal(body, state.PayloadTypePlan, encrypters.Noop{})
				require.NoError(t, err)
				_, err = runtime.OpenPlanV2(sealed, func(*runtime.StateRef) (encrypt.Encrypter, error) {
					return encrypters.Noop{}, nil
				})
				require.ErrorContains(t, err, "obsolete alpha format")
				requirePriorTarget(t, executor, prior)
			}
			requireFileContent(t, path, "original")
		})
	}
}
