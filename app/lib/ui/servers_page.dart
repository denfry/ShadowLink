// SPDX-License-Identifier: GPL-3.0-only
import 'package:flutter/material.dart';
import '../ffi/core.dart';

class ServersPage extends StatefulWidget {
  final CoreApi core;
  const ServersPage({super.key, required this.core});
  @override
  State<ServersPage> createState() => _ServersPageState();
}

class _ServersPageState extends State<ServersPage> {
  final _input = TextEditingController();
  String? _error;

  @override
  Widget build(BuildContext context) {
    final servers = widget.core.listServers();
    final selected = widget.core.selected();
    return Scaffold(
      appBar: AppBar(title: const Text('Servers')),
      body: Column(children: [
        Padding(
          padding: const EdgeInsets.all(12),
          child: Row(children: [
            Expanded(
              child: TextField(
                controller: _input,
                decoration: const InputDecoration(
                    labelText: 'vless:// link, file path, or subscription body'),
              ),
            ),
            const SizedBox(width: 8),
            FilledButton(
              onPressed: () {
                try {
                  widget.core.import(_input.text.trim());
                  _input.clear();
                  setState(() => _error = null);
                } on CoreException catch (e) {
                  setState(() => _error = e.message);
                }
              },
              child: const Text('Import'),
            ),
          ]),
        ),
        if (_error != null) Text(_error!, style: const TextStyle(color: Colors.red)),
        Expanded(
          child: ListView(
            children: servers
                .map((s) => ListTile(
                      leading: Icon(s.tag == selected
                          ? Icons.radio_button_checked
                          : Icons.radio_button_unchecked),
                      title: Text(s.masked),
                      onTap: () {
                        try {
                          widget.core.select(s.tag);
                          setState(() => _error = null);
                        } on CoreException catch (e) {
                          setState(() => _error = e.message);
                        }
                      },
                    ))
                .toList(),
          ),
        ),
      ]),
    );
  }
}
