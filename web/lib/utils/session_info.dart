import 'dart:convert';

import 'package:jaspr/jaspr.dart';
import 'package:universal_web/web.dart' as web;

import 'token_storage.dart';

/// What a session token says about who is signed in.
///
/// This is decoded straight out of the token so the UI can show a name next to
/// the logout button. The signature is deliberately not checked here, and
/// nothing in it may drive a decision: the server is the only thing that
/// decides what a token is allowed to do.
class SessionInfo {
  const SessionInfo({
    this.subject,
    this.email,
    this.name,
    this.username,
    this.via,
  });

  final String? subject;
  final String? email;
  final String? name;
  final String? username;

  /// How the token was obtained: "oidc" for a provider login, "mint" for one
  /// created with the `token` command. Null when the token is not a JWT at all,
  /// which is the usual case for the static API token.
  final String? via;

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
      );
    } catch (e) {
      // A token that isn't a readable JWT is expected here, not an error.
      return null;
    }
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
    for (final candidate in [email, name, username, subject]) {
      if (candidate != null && candidate.isNotEmpty) {
        return candidate;
      }
    }
    return null;
  }
}
