# Standard library

The unobin standard library provides resources and actions for common
tasks such as operating on files, generating identifiers, running processes,
and making HTTP requests.

```
factory: {
  description: 'Writes an app config file.'

  inputs: {
    config-path: { type: string }
    app-name:    { type: string }
  }

  imports: { std: 'github.com/cloudboss/unobin-library-std' }

  resources: {
    config: std.fs-file {
      path: input.config-path
      content: @core.to-json({ app: input.app-name })
      create-directory: true
    }
  }

  outputs: {
    config-sha256: { value: resource.config.sha256 }
  }
}
```

This branch targets std `v0.5.0-a.1` with Unobin `v0.12.0-a.3` and Go
`1.26.2`. After the std release is published, add it to the dependency
project before compiling the factory:

```
unobin deps get github.com/cloudboss/unobin-library-std@v0.5.0-a.1
```

Create a zip archive from a directory:

```
resources: {
  package: std.archive-zipfile {
    path: './build/app.zip'
    source-dir: './app'
    create-directory: true
    excludes: ['**/.git/**']
  }
}
```

Generate an identifier that remains stable until the resource is replaced:

```
resources: {
  suffix: std.random-id {
    byte-length: 8
    prefix: 'web-'
  }
}

outputs: {
  name: { value: resource.suffix.hex }
}
```

Changing `byte-length`, `prefix`, or any value in `keepers` replaces the
resource and generates a new identifier.

Adding or removing a keeper also replaces the ID. Omitted optional values
remain distinct from explicitly empty maps or strings. Reordering equal
keeper entries leaves the ID unchanged. All five encodings persist in state;
`id` is the unprefixed identifier, while the other encodings include `prefix`.

## Upgrading from std v0.4.0 and earlier

The new runtime uses format 2 for saved plans and state. It rejects format-1
artifacts with an obsolete-format error. This library does not convert old
plans or state, and resource schema migration does not upgrade those formats.

Keep the prior toolchain and state available to manage or remove existing
objects. Alternatively, choose a deliberate fresh-state deployment with
appropriate file destinations. Removing state alone neither deletes nor
adopts existing files. Creating `random-id` with fresh state generates new
values, so account for any consumers that depend on the old identifiers.

File and archive destination changes replace the resource: apply deletes the
recorded destination and creates the desired one. Changes to file content or
mode, archive entries, or declared source paths update the existing destination.
Updated checksums and sizes become available during apply, and dependent
resources and triggered actions use those new values.

Before creating, updating, or replacing a resource, apply validates its new
inputs. Archive checks include entry names, duplicate entries, empty archives,
and source availability. A validation error leaves the prior resource intact.
Later I/O failures or source changes can still occur after validation; there
is no rollback guarantee after deletion. Inspect the error, destination files,
and recorded state before creating another plan.

Changes to bytes behind an unchanged archive `source-dir` or `source-file`
input are not detected. Filesystem permission drift is also not detected.
Relative paths are resolved from the process working directory.

## Checking a source checkout

Run `make test` or `go test -short ./...` for direct operations, saved plans,
and state reloads. Run `make test-all` or `go test ./...` to also check a
compiled factory that uses all seven exports and HTTP against a local server.
The compiled test uses Unobin's public `pkg/e2etest` framework, which builds
the factory directly and reuses Go's caches. Its first run may download
missing dependencies or the Go toolchain.

Run `make test-release` for the separate distribution check. It packages this
checkout in a temporary Go module proxy, downloads dependencies into an empty
module cache, and builds and runs the
same consumer without module replacements. This checks the unpublished std
package against the published Unobin tag; it does not publish a release.
This test requires the `release` build tag and is excluded from ordinary runs.

Run `make docs` to generate the reference. Local generation and CI both use
docgen `v0.2.1`; generated files are not committed.

## Configuration

The standard library has no library configuration.

## Reference

The generated reference lists every resource and action kind, its inputs,
outputs, defaults, and sensitive fields.

- [File](reference/resources/fs-file.md)
- [Zip archive](reference/resources/archive-zipfile.md)
- [Random ID](reference/resources/random-id.md)
- [Command](reference/actions/exec-command.md)
- [Script](reference/actions/exec-script.md)
- [Wait for command](reference/actions/exec-wait-for.md)
- [HTTP request](reference/actions/net-http.md)
