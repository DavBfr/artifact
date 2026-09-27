import 'dart:convert';

import 'package:jaspr/jaspr.dart';
import 'package:universal_web/web.dart' as web;

import 'token_storage.dart';

/// What a session token says about who is signed in.
///
/// This is decoded straight out of the token so the UI can show a name next to
/// the logout button, and so it can hide controls the server would refuse
/// anyway. The signature is deliberately not checked here, and nothing read here
/// is a security decision: the server is the only thing that decides what a
/// token is allowed to do, and a refusal is still handled when it arrives.
class SessionInfo {
  const SessionInfo({
    this.subject,
    this.email,
    this.name,
    this.username,
    this.via,
    this.permissions = const <String>{},
  });

  /// The permissions the server enforces. They are a compatibility surface -
  /// they travel inside issued tokens - so they are added, never renamed.
  static const String fileCreate = 'file:create';
  static const String fileDelete = 'file:delete';
  static const String tagAdd = 'tag:add';
  static const String tagRemove = 'tag:remove';

  /// Every permission, which is what a credential that is not one of this
  /// server's JWTs - the static API token - always carries.
  static const Set<String> allPermissions = {
    fileCreate,
    fileDelete,
    tagAdd,
    tagRemove,
  };

  final String? subject;
  final String? email;
  final String? name;
  final String? username;

  /// How the token was obtained: "oidc" for a provider login, "mint" for one
  /// created with the `token` command. Null when the token is not a JWT at all,
  /// which is the usual case for the static API token.
  final String? via;

  /// What the token grants, resolved by the server at login from its roles and
  /// the provider's groups. A token with no `perms` claim - one minted before
  /// permissions existed - yields an empty set, which is also what the server
  /// makes of it: it can read, and nothing else.
  final Set<String> permissions;

  /// Whether the token grants permission.
  bool hasPermission(String permission) => permissions.contains(permission);

  /// Decodes the payload of [token], or returns null when it is not a JWT whose
  /// payload can be read.
  static SessionInfo? fromToken(String? token) {
    if (token == null || token.isEmpty) {
      return null;
    }

    final parts = token.split('.');
    if (parts.length != 3) {
      return null;
    }

    try {
      final decoded = utf8.decode(
        base64Url.decode(base64Url.normalize(parts[1])),
      );
      final payload = jsonDecode(decoded);
      if (payload is! Map) {
        return null;
      }

      return SessionInfo(
        subject: payload['sub'] as String?,
        email: payload['email'] as String?,
        name: payload['name'] as String?,
        username: payload['preferred_username'] as String?,
        via: payload['via'] as String?,
        permissions: _permissionsFrom(payload['perms']),
      );
    } catch (e) {
      // A token that isn't a readable JWT is expected here, not an error.
      return null;
    }
  }

  /// Reads the permission list out of the claim, ignoring anything that is not a
  /// string so a malformed token degrades to "no permissions" rather than
  /// throwing.
  static Set<String> _permissionsFrom(Object? raw) {
    if (raw is! List) {
      return const <String>{};
    }
    return raw.whereType<String>().toSet();
  }

  /// Reads the result of a provider login out of the URL fragment, storing the
  /// token when there is one and clearing the fragment either way.
  ///
  /// Returns the server's error code when the login failed, and null otherwise -
  /// including when there was nothing to consume. Must run before anything
  /// derives a URL from the page location, because the fragment is part of it.
  static String? consumeLoginFragment(BuildContext context) {
    String fragment;
    try {
      fragment = web.window.location.hash;
    } catch (e) {
      return null;
    }

    if (fragment.length < 2) {
      return null;
    }

    final params = Uri.splitQueryString(fragment.substring(1));
    final token = params['token'];
    final error = params['auth_error'];

    if (token == null && error == null) {
      return null;
    }

    // Clear it whichever way it went: a token should not linger in the address
    // bar (or be re-consumed by a reload), and a stale error should not come
    // back on the next visit.
    _clearFragment();

    if (error != null && error.isNotEmpty) {
      return error;
    }

    if (token != null && token.isNotEmpty) {
      TokenStorage.saveToken(context, token);
    }

    return null;
  }

  /// Rewrites the address bar without the fragment. Failures are ignored: the
  /// worst case is that the fragment stays visible.
  static void _clearFragment() {
    try {
      final location = web.window.location;
      web.window.history.replaceState(
        null,
        '',
        '${location.pathname}${location.search}',
      );
    } catch (e) {
      // Nothing to do - the login result has already been applied.
    }
  }

  /// The best available human-readable name for the signed-in user, falling
  /// back to the subject when the token carries no profile claims.
  String? get label {
    for (final candidate in [name, username, email, subject]) {
      if (candidate != null && candidate.isNotEmpty) {
        return candidate;
      }
    }
    return null;
  }
}
