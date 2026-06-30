package std

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudboss/unobin/pkg/goschema"
	"github.com/cloudboss/unobin/pkg/lang"
	"github.com/cloudboss/unobin/pkg/typecheck"
)

// TestSchemaDeclaresInputsAndDefaults reads this library the way the compiler
// does and asserts the inputs and defaults that should be visible to factories.
func TestSchemaDeclaresInputsAndDefaults(t *testing.T) {
	schema, warnings, err := goschema.Read(".")
	require.NoError(t, err)
	require.Empty(t, warnings)

	require.Equal(t, []lang.DefaultSpec{
		{Field: "input.mode", Value: "420"},
	}, schema.Resources["archive-zipfile"].Defaults)
	requireInputType(t, schema.Resources["archive-zipfile"].Inputs,
		"create-directory", typecheck.TOptional(typecheck.TBoolean()))
	requireInputType(t, schema.Resources["archive-zipfile"].Inputs,
		"source-dir", typecheck.TOptional(typecheck.TString()))
	requireInputType(t, schema.Resources["archive-zipfile"].Inputs,
		"source-file", typecheck.TOptional(typecheck.TString()))
	requireInputType(t, schema.Resources["archive-zipfile"].Inputs,
		"excludes", optionalStringList())
	requireInputType(t, schema.Resources["archive-zipfile"].Inputs,
		"entry-mode", typecheck.TOptional(typecheck.TInteger()))

	require.Equal(t, []lang.DefaultSpec{
		{Field: "input.mode", Value: "420"},
	}, schema.Resources["fs-file"].Defaults)
	requireInputType(t, schema.Resources["fs-file"].Inputs,
		"create-directory", typecheck.TOptional(typecheck.TBoolean()))

	require.Empty(t, schema.Actions["exec-command"].Defaults)
	requireInputType(t, schema.Actions["exec-command"].Inputs,
		"environment", optionalStringMap())
	requireInputType(t, schema.Actions["exec-command"].Inputs,
		"working-dir", typecheck.TOptional(typecheck.TString()))

	require.Equal(t, []lang.DefaultSpec{
		{Field: "input.shell", Value: "'sh'"},
	}, schema.Actions["exec-script"].Defaults)
	requireInputType(t, schema.Actions["exec-script"].Inputs,
		"environment", optionalStringMap())
	requireInputType(t, schema.Actions["exec-script"].Inputs,
		"working-dir", typecheck.TOptional(typecheck.TString()))

	require.Empty(t, schema.Actions["net-http"].Defaults)
	requireInputType(t, schema.Actions["net-http"].Inputs,
		"method", typecheck.TOptional(typecheck.TString()))
	requireInputType(t, schema.Actions["net-http"].Inputs,
		"headers", optionalStringMap())
	requireInputType(t, schema.Actions["net-http"].Inputs,
		"body", typecheck.TOptional(typecheck.TString()))
	requireInputType(t, schema.Actions["net-http"].Inputs,
		"timeout", typecheck.TOptional(typecheck.TInteger()))

	require.Equal(t, []lang.DefaultSpec{
		{Field: "input.interval", Value: "1000000000"},
		{Field: "input.timeout", Value: "300000000000"},
	}, schema.Actions["exec-wait-for"].Defaults)
	requireInputType(t, schema.Actions["exec-wait-for"].Inputs,
		"environment", optionalStringMap())
	requireInputType(t, schema.Actions["exec-wait-for"].Inputs,
		"working-dir", typecheck.TOptional(typecheck.TString()))
}

func optionalStringMap() typecheck.Type {
	return typecheck.TOptional(typecheck.TMap(typecheck.TString()))
}

func optionalStringList() typecheck.Type {
	return typecheck.TOptional(typecheck.TList(typecheck.TString()))
}

func requireInputType(
	t *testing.T,
	inputs map[string]typecheck.Type,
	name string,
	want typecheck.Type,
) {
	t.Helper()
	got, ok := inputs[name]
	require.True(t, ok)
	require.Truef(t, got.Equal(want), "input %s: got %s, want %s", name, got, want)
}
