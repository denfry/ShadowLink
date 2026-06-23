// SPDX-License-Identifier: GPL-3.0-only

/// ConnStatus is the parsed result of SL_Status.
class ConnStatus {
  final String state; // disconnected | connecting | connected | reconnecting | disconnecting | error
  final String server; // masked
  final int delayMs;
  final String error;
  const ConnStatus(
      {required this.state, this.server = '', this.delayMs = 0, this.error = ''});

  factory ConnStatus.fromJson(Map<String, dynamic> m) => ConnStatus(
        state: (m['state'] ?? 'disconnected') as String,
        server: (m['server'] ?? '') as String,
        delayMs: (m['delayMs'] ?? 0) as int,
        error: (m['error'] ?? '') as String,
      );

  bool get connected => state == 'connected';
  static const disconnected = ConnStatus(state: 'disconnected');
}

/// ServerInfo is one entry from SL_ListServers.
class ServerInfo {
  final String tag;
  final String host;
  final String masked;
  const ServerInfo({required this.tag, required this.host, required this.masked});
  factory ServerInfo.fromJson(Map<String, dynamic> m) => ServerInfo(
      tag: m['tag'] as String, host: m['host'] as String, masked: m['masked'] as String);
}

/// Thrown when a core call returns {"ok":false}.
class CoreException implements Exception {
  final String message;
  final bool needsAdmin;
  CoreException(this.message, {this.needsAdmin = false});
  @override
  String toString() => 'CoreException($message, needsAdmin=$needsAdmin)';
}

/// CoreApi is the engine boundary. ShadowlinkCore (FFI) is the real impl;
/// FakeCore drives widget/controller tests without a DLL.
abstract class CoreApi {
  String version();
  List<ServerInfo> listServers();
  String selected();
  List<ServerInfo> import(String input);
  void select(String tag);
  void start({bool failOpen = false, String? tag});
  void stop();
  ConnStatus status();
}

/// FakeCore is an in-memory CoreApi for tests.
class FakeCore implements CoreApi {
  final List<ServerInfo> _servers = [];
  String _selected = '';
  bool _connected = false;
  bool failOnStart = false;

  @override
  String version() => '0.0.0-fake';
  @override
  List<ServerInfo> listServers() => List.unmodifiable(_servers);
  @override
  String selected() => _selected;
  @override
  List<ServerInfo> import(String input) {
    _servers.add(const ServerInfo(tag: 'n1', host: 'ex.com', masked: 'n1 (ex.com:443 uuid=uu** pbk=K***)'));
    if (_selected.isEmpty) _selected = 'n1';
    return listServers();
  }

  @override
  void select(String tag) {
    if (!_servers.any((s) => s.tag == tag)) {
      throw CoreException('no server with tag $tag');
    }
    _selected = tag;
  }

  @override
  void start({bool failOpen = false, String? tag}) {
    if (failOnStart) throw CoreException('boom', needsAdmin: true);
    if (_servers.isEmpty) throw CoreException('no servers; import one first');
    _connected = true;
  }

  @override
  void stop() => _connected = false;
  @override
  ConnStatus status() => _connected
      ? const ConnStatus(state: 'connected', server: 'n1 (ex.com:443 ...)', delayMs: 42)
      : ConnStatus.disconnected;
}
