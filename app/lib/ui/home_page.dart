// SPDX-License-Identifier: GPL-3.0-only
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
                  child: Text(
                    c.needsAdmin
                        ? 'Run as administrator to bring up the tunnel.'
                        : 'Error: ${c.lastError}',
                    style: const TextStyle(color: Colors.red),
                  ),
                ),
            ]),
          );
        },
      ),
    );
  }
}
