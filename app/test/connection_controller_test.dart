// SPDX-License-Identifier: GPL-3.0-only
import 'package:flutter_test/flutter_test.dart';
import 'package:shadowlink/ffi/core.dart';
import 'package:shadowlink/state/connection_controller.dart';

void main() {
  test('connect reaches connected, disconnect returns to disconnected', () async {
    final core = FakeCore();
    core.import('vless://x');
    final c = ConnectionController(core);

    await c.connect();
    expect(c.status.connected, isTrue);

    await c.disconnect();
    expect(c.status.state, 'disconnected');
  });

  test('start failure surfaces needsAdmin without crashing', () async {
    final core = FakeCore()..failOnStart = true;
    core.import('vless://x');
    final c = ConnectionController(core);

    await c.connect();
    expect(c.status.connected, isFalse);
    expect(c.needsAdmin, isTrue);
    expect(c.lastError, isNotNull);
  });
}
