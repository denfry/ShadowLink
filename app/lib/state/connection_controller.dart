// SPDX-License-Identifier: GPL-3.0-only
import 'dart:async';
import 'package:flutter/foundation.dart';
import '../ffi/core.dart';

/// ConnectionController owns the live connection state and polls SL_Status.
class ConnectionController extends ChangeNotifier {
  final CoreApi core;
  ConnStatus status = ConnStatus.disconnected;
  String? lastError;
  bool needsAdmin = false;
  Timer? _poll;

  ConnectionController(this.core);

  Future<void> connect({bool failOpen = false}) async {
    lastError = null;
    needsAdmin = false;
    try {
      await Future(() => core.start(failOpen: failOpen));
      _startPolling();
    } on CoreException catch (e) {
      lastError = e.message;
      needsAdmin = e.needsAdmin;
    }
    refresh();
  }

  Future<void> disconnect() async {
    _poll?.cancel();
    _poll = null;
    try {
      await Future(() => core.stop());
    } on CoreException catch (e) {
      lastError = e.message;
    }
    refresh();
  }

  void refresh() {
    try {
      status = core.status();
    } on CoreException catch (e) {
      lastError = e.message;
    }
    notifyListeners();
  }

  void _startPolling() {
    _poll?.cancel();
    _poll = Timer.periodic(const Duration(seconds: 2), (_) => refresh());
  }

  @override
  void dispose() {
    _poll?.cancel();
    super.dispose();
  }
}
