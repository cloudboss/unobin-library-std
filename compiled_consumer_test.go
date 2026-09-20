package std

import (
	"testing"

	"github.com/cloudboss/unobin/pkg/e2etest"
)

func TestCompiledConsumer(t *testing.T) {
	e2etest.RunCompiledCases(t, "testdata/ub/compiled/valid",
		e2etest.WithGoModule(libraryPath, "."),
	)
}
