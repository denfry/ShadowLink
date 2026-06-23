// SPDX-License-Identifier: GPL-3.0-only

//go:build libshadowlink

package main

/*
#include <stdlib.h>
*/
import "C"

import (
	"unsafe"

	"github.com/shadowlink/shadowlink/internal/ffiapi"
)

var api = ffiapi.Default()

//export SL_Version
func SL_Version() *C.char { return C.CString(api.Version()) }

//export SL_ListServers
func SL_ListServers() *C.char { return C.CString(api.ListServers()) }

//export SL_Import
func SL_Import(input *C.char) *C.char { return C.CString(api.Import(C.GoString(input))) }

//export SL_Select
func SL_Select(tag *C.char) *C.char { return C.CString(api.Select(C.GoString(tag))) }

//export SL_Start
func SL_Start(opts *C.char) *C.char { return C.CString(api.Start(C.GoString(opts))) }

//export SL_Stop
func SL_Stop() *C.char { return C.CString(api.Stop()) }

//export SL_Status
func SL_Status() *C.char { return C.CString(api.Status()) }

//export SL_FreeString
func SL_FreeString(p *C.char) { C.free(unsafe.Pointer(p)) }

func main() {}
