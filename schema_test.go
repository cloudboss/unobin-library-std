package std

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudboss/unobin/pkg/goschema"
	"github.com/cloudboss/unobin/pkg/lang"
)

// TestSchemaDeclaresDefaults reads this library the way the compiler
// does and asserts each type's declared defaults, so an extraction
// warning or a divergence between code and declaration fails here
// first.
func TestSchemaDeclaresDefaults(t *testing.T) {
	schema, warnings, err := goschema.Read(".")
	require.NoError(t, err)
	require.Empty(t, warnings)

	require.Equal(t, []lang.DefaultSpec{
		{Field: "input.mode", Value: "420"},
		{Field: "input.create-directory", Optional: true},
	}, schema.Resources["fs-file"].Defaults)

	require.Equal(t, []lang.DefaultSpec{
		{Field: "input.environment", Optional: true},
		{Field: "input.working-dir", Optional: true},
	}, schema.Actions["exec-command"].Defaults)

	require.Equal(t, []lang.DefaultSpec{
		{Field: "input.shell", Value: "'sh'"},
		{Field: "input.environment", Optional: true},
		{Field: "input.working-dir", Optional: true},
	}, schema.Actions["exec-script"].Defaults)

	require.Equal(t, []lang.DefaultSpec{
		{Field: "input.method", Value: "'GET'"},
		{Field: "input.headers", Optional: true},
		{Field: "input.body", Optional: true},
		{Field: "input.timeout", Optional: true},
	}, schema.Actions["net-http"].Defaults)

	require.Equal(t, []lang.DefaultSpec{
		{Field: "input.interval", Value: "1000000000"},
		{Field: "input.timeout", Value: "300000000000"},
		{Field: "input.environment", Optional: true},
		{Field: "input.working-dir", Optional: true},
	}, schema.Actions["exec-wait-for"].Defaults)
}
