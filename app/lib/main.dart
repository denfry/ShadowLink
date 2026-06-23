// SPDX-License-Identifier: GPL-3.0-only
import 'package:flutter/material.dart';
import 'ffi/core.dart';
import 'ffi/core_ffi.dart';
import 'state/connection_controller.dart';
import 'ui/home_page.dart';

void main() {
  // Swap ShadowlinkCore.open() for FakeCore() to run the UI without the DLL.
  final CoreApi core = ShadowlinkCore.open();
  runApp(ShadowLinkApp(controller: ConnectionController(core)));
}

class ShadowLinkApp extends StatelessWidget {
  final ConnectionController controller;
  const ShadowLinkApp({super.key, required this.controller});

  @override
  Widget build(BuildContext context) => MaterialApp(
        title: 'ShadowLink',
        theme: ThemeData(colorSchemeSeed: Colors.indigo, useMaterial3: true),
        home: HomePage(controller: controller),
      );
}
