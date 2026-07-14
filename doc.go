// Package std hosts the standard actions and resources that touch the
// world: process execution, HTTP, and local files. The language's pure
// functions live in the @core namespace, provided by the toolchain
// with no import; everything here is imported and versioned like any
// other library.
//
// Actions:
//   - std.exec-command - exec a process, capture stdout/stderr/exit
//   - std.exec-script - multi-line script via `<shell> -c` (defaults to sh)
//   - std.net-http - HTTP request, return body/status
//   - std.exec-wait-for - poll a command until it exits 0 or the deadline
//
// Resources:
//   - std.archive-zipfile - a zip archive on the local filesystem
//   - std.fs-file - a regular file on the local filesystem
//   - std.random-id - a cryptographically random identifier
//
// Actions implement the standard action interface: triggered
// (hash-based re-run), with @lock cross-DAG serialization, @timeout,
// and @sensitive redaction.
package std
