import 'package:jaspr/jaspr.dart';
import 'package:universal_web/web.dart' as web;

import '../bulma/bulma.dart';

/// Utility class for managing authentication token in browser's local storage
class TokenStorage {
  static const String _tokenKey = 'authToken';

  /// Save token to local storage
  static void saveToken(BuildContext context, String token) {
    try {
      web.window.localStorage.setItem(_tokenKey, token);
    } catch (e) {
      NotificationMessenger.of(context).showNotification(
        BulmaNotification.error('Failed to save token to local storage: $e'),
      );
    }
  }

  /// Get token from local storage
  static String? getToken(BuildContext context) {
    try {
      return web.window.localStorage.getItem(_tokenKey);
    } catch (e) {
      NotificationMessenger.of(context).showNotification(
        BulmaNotification.error('Failed to get token from local storage: $e'),
      );
      return null;
    }
  }

  /// Remove token from local storage
  static void removeToken(BuildContext context) {
    try {
      web.window.localStorage.removeItem(_tokenKey);
    } catch (e) {
      NotificationMessenger.of(context).showNotification(
        BulmaNotification.error(
          'Failed to remove token from local storage: $e',
        ),
      );
    }
  }

  /// Check if token exists in local storage
  static bool hasToken(BuildContext context) {
    final token = getToken(context);
    return token != null && token.isNotEmpty;
  }

  /// Clear all stored data (for testing/debugging)
  static void clearAll(BuildContext context) {
    try {
      web.window.localStorage.clear();
    } catch (e) {
      NotificationMessenger.of(context).showNotification(
        BulmaNotification.error('Failed to clear local storage: $e'),
      );
    }
  }
}
