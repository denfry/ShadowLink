// SPDX-License-Identifier: GPL-3.0-only
import 'dart:convert';
import 'dart:ffi';
import 'dart:io';
import 'package:ffi/ffi.dart';
import 'core.dart';

typedef _NoArgC = Pointer<Utf8> Function();
typedef _NoArgD = Pointer<Utf8> Function();
typedef _StrArgC = Pointer<Utf8> Function(Pointer<Utf8>);
typedef _StrArgD = Pointer<Utf8> Function(Pointer<Utf8>);
typedef _FreeC = Void Function(Pointer<Utf8>);
typedef _FreeD = void Function(Pointer<Utf8>);

/// ShadowlinkCore is the real CoreApi, calling shadowlink_core.dll via dart:ffi.
class ShadowlinkCore implements CoreApi {
  final DynamicLibrary _lib;
  late final _NoArgD _version = _lib.lookupFunction<_NoArgC, _NoArgD>('SL_Version');
  late final _NoArgD _list = _lib.lookupFunction<_NoArgC, _NoArgD>('SL_ListServers');
  late final _StrArgD _import = _lib.lookupFunction<_StrArgC, _StrArgD>('SL_Import');
  late final _StrArgD _select = _lib.lookupFunction<_StrArgC, _StrArgD>('SL_Select');
  late final _StrArgD _start = _lib.lookupFunction<_StrArgC, _StrArgD>('SL_Start');
  late final _NoArgD _stop = _lib.lookupFunction<_NoArgC, _NoArgD>('SL_Stop');
  late final _NoArgD _status = _lib.lookupFunction<_NoArgC, _NoArgD>('SL_Status');
  late final _FreeD _free = _lib.lookupFunction<_FreeC, _FreeD>('SL_FreeString');

  ShadowlinkCore._(this._lib);

  /// Loads shadowlink_core.dll from beside the executable.
  factory ShadowlinkCore.open() {
    final name = Platform.isWindows
        ? 'shadowlink_core.dll'
        : Platform.isMacOS
            ? 'libshadowlink_core.dylib'
            : 'libshadowlink_core.so';
    return ShadowlinkCore._(DynamicLibrary.open(name));
  }

  Map<String, dynamic> _call(Pointer<Utf8> ptr) {
    try {
      final s = ptr.toDartString();
      final m = jsonDecode(s) as Map<String, dynamic>;
      if (m['ok'] != true) {
        throw CoreException((m['error'] ?? 'unknown error') as String,
            needsAdmin: m['needsAdmin'] == true);
      }
      return m;
    } finally {
      _free(ptr);
    }
  }

  Pointer<Utf8> _withStr(_StrArgD fn, String arg) {
    final p = arg.toNativeUtf8();
    try {
      return fn(p);
    } finally {
      calloc.free(p);
    }
  }

  @override
  String version() => _call(_version())['version'] as String;
  @override
  List<ServerInfo> listServers() => ((_call(_list())['servers']) as List)
      .map((e) => ServerInfo.fromJson(e as Map<String, dynamic>))
      .toList();
  @override
  String selected() => (_call(_list())['selected'] ?? '') as String;
  @override
  List<ServerInfo> import(String input) => ((_call(_withStr(_import, input))['servers']) as List)
      .map((e) => ServerInfo.fromJson(e as Map<String, dynamic>))
      .toList();
  @override
  void select(String tag) => _call(_withStr(_select, tag));
  @override
  void start({bool failOpen = false, String? tag}) =>
      _call(_withStr(_start, jsonEncode({'failOpen': failOpen, if (tag != null) 'tag': tag})));
  @override
  void stop() => _call(_stop());
  @override
  ConnStatus status() => ConnStatus.fromJson(_call(_status()));
}
