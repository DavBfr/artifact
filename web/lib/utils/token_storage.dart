import 'package:universal_web/web.dart' as web;

/// Utility class for managing authentication token in browser's local storage
class TokenStorage {
  static const String _tokenKey = 'artifact_api_token';

  /// Save token to local storage
  static void saveToken(String token) {
    try {
      web.window.localStorage.setItem(_tokenKey, token);
    } catch (e) {
      print('Failed to save token to local storage: $e');
    }
  }

  /// Get token from local storage
  static String? getToken() {
    try {
      return web.window.localStorage.getItem(_tokenKey);
    } catch (e) {
      print('Failed to get token from local storage: $e');
      return null;
    }
  }

  /// Remove token from local storage
  static void removeToken() {
    try {
      web.window.localStorage.removeItem(_tokenKey);
    } catch (e) {
      print('Failed to remove token from local storage: $e');
    }
  }

  /// Check if token exists in local storage
  static bool hasToken() {
    final token = getToken();
    return token != null && token.isNotEmpty;
  }

  /// Clear all stored data (for testing/debugging)
  static void clearAll() {
    try {
      web.window.localStorage.clear();
    } catch (e) {
      print('Failed to clear local storage: $e');
    }
  }
}
