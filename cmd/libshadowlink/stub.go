// SPDX-License-Identifier: GPL-3.0-only

//go:build !libshadowlink

// Package main is the c-shared entrypoint. Without the `libshadowlink` build
// tag it is an empty stub so default `go build ./...` and CI stay green in
// environments without a 64-bit C toolchain. The real cgo shim (shim.go) builds
// only with `-tags libshadowlink`.
package main

func main() {}
