// SPDX-License-Identifier: GPL-3.0-only
import 'dart:io';
import 'package:flutter/material.dart';
import '../state/connection_controller.dart';
import 'servers_page.dart';

class HomePage extends StatefulWidget {
  final ConnectionController controller;
  const HomePage({super.key, required this.controller});
  @override
  State<HomePage> createState() => _HomePageState();
}

class _HomePageState extends State<HomePage> {
  bool _killSwitch = true;

  @override
  void initState() {
    super.initState();
    widget.controller.refresh();
  }

  /// Relaunch this executable elevated (UAC) via PowerShell, then exit so only
  /// the elevated instance remains. Only reachable when launched non-elevated
  /// (the packaged app's manifest already requests requireAdministrator).
  Future<void> _relaunchElevated() async {
    final exe = Platform.resolvedExecutable;
    await Process.start('powershell', [
      '-NoProfile',
      '-Command',
      "Start-Process -FilePath '$exe' -Verb RunAs",
    ]);
    exit(0);
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('ShadowLink'), actions: [
        IconButton(
          icon: const Icon(Icons.dns),
          tooltip: 'Servers',
          onPressed: () => Navigator.push(context,
              MaterialPageRoute(builder: (_) => ServersPage(core: widget.controller.core))),
        ),
      ]),
      body: ListenableBuilder(
        listenable: widget.controller,
        builder: (context, _) {
          final c = widget.controller;
          final st = c.status;
          return Center(
            child: Column(mainAxisAlignment: MainAxisAlignment.center, children: [
              Text(st.state.toUpperCase(),
                  style: Theme.of(context).textTheme.headlineSmall),
              const SizedBox(height: 24),
              FilledButton(
                onPressed: () => st.connected
                    ? c.disconnect()
                    : c.connect(failOpen: !_killSwitch),
                child: Text(st.connected ? 'DISCONNECT' : 'CONNECT'),
              ),
              const SizedBox(height: 16),
              if (st.connected) Text('${st.server} · ${st.delayMs} ms'),
              SwitchListTile(
                title: const Text('Kill-switch'),
                value: _killSwitch,
                onChanged: st.connected ? null : (v) => setState(() => _killSwitch = v),
              ),
              if (c.lastError != null)
                Padding(
                  padding: const EdgeInsets.all(12),
                  child: c.needsAdmin
                      ? Column(children: [
                          const Text('Run as administrator to bring up the tunnel.',
                              style: TextStyle(color: Colors.red)),
                          const SizedBox(height: 8),
                          FilledButton(
                            onPressed: _relaunchElevated,
                            child: const Text('Relaunch as administrator'),
                          ),
                        ])
                      : Text('Error: ${c.lastError}',
                          style: const TextStyle(color: Colors.red)),
                ),
            ]),
          );
        },
      ),
    );
  }
}
