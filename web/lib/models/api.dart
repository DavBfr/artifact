import 'dart:async';
import 'dart:convert';

import 'package:http/http.dart' as http;
import 'package:jaspr/jaspr.dart';
import 'package:universal_web/js_interop.dart';
import 'package:universal_web/web.dart' as web;

import 'api_models.dart';

/// API client for communicating with the artifact server
class ArtifactApiClient {
  ArtifactApiClient({required this.baseUrl, this.authToken});

  factory ArtifactApiClient.base({String? authToken}) {
    final baseUrl = kDebugMode
        ? 'http://127.0.0.1:9080'
        : Uri.base.toString().replaceAll(RegExp(r'\/$'), '');
    return ArtifactApiClient(baseUrl: baseUrl, authToken: authToken);
  }

  final String baseUrl;
  final String? authToken;

  bool get isAuthenticated => authToken != null && authToken!.isNotEmpty;

  /// Get authorization headers
  Map<String, String> get _headers {
    final headers = <String, String>{'Content-Type': 'application/json'};
    if (authToken != null && authToken!.isNotEmpty) {
      headers['Authorization'] = 'Bearer $authToken';
    }
    return headers;
  }

  /// Health check endpoint
  Future<HealthResponse> healthCheck() async {
    final response = await http.get(Uri.parse('$baseUrl/api/health'));

    if (response.statusCode == 200) {
      return HealthResponse.fromJson(jsonDecode(response.body));
    } else {
      throw ApiException(
        'Health check failed',
        statusCode: response.statusCode,
      );
    }
  }

  /// List all files
  Future<ListFilesResponse> listFiles() async {
    final response = await http.get(Uri.parse('$baseUrl/api/files'));

    if (response.statusCode == 200) {
      return ListFilesResponse.fromJson(jsonDecode(response.body));
    } else {
      throw ApiException(
        'Failed to list files',
        statusCode: response.statusCode,
      );
    }
  }

  /// Get server configuration
  Future<ConfigResponse> getConfig() async {
    final response = await http.get(
      Uri.parse('$baseUrl/api/config'),
      headers: _headers,
    );

    if (response.statusCode == 200) {
      return ConfigResponse.fromJson(jsonDecode(response.body));
    } else if (response.statusCode == 401) {
      throw AuthenticationException('Authentication required');
    } else {
      throw ApiException(
        'Failed to get config',
        statusCode: response.statusCode,
      );
    }
  }

  /// Upload a file with progress tracking using XMLHttpRequest (web only)
  Future<UploadResponse> uploadFile({
    required web.File file,
    void Function(int sent, int total)? onProgress,
  }) async {
    if (authToken == null || authToken!.isEmpty) {
      throw AuthenticationException('Authentication token required');
    }

    final formData = web.FormData();
    formData.append('file', file);

    final xhr = web.XMLHttpRequest();
    xhr.open('POST', '$baseUrl/api/upload');
    xhr.setRequestHeader('Authorization', 'Bearer $authToken');

    // Track upload progress
    if (onProgress != null) {
      xhr.upload.addEventListener(
        'progress',
        ((web.Event event) {
          final progressEvent = event as web.ProgressEvent;
          if (progressEvent.lengthComputable) {
            onProgress(progressEvent.loaded, progressEvent.total);
          }
        }).toJS,
      );
    }

    // Create a completer to handle the async response
    final completer = Completer<UploadResponse>();

    xhr.addEventListener(
      'load',
      ((web.Event event) {
        if (xhr.status == 200 || xhr.status == 201) {
          try {
            final data = jsonDecode(xhr.responseText);
            completer.complete(UploadResponse.fromJson(data));
          } catch (e) {
            completer.completeError(
              ApiException('Failed to parse response: $e'),
            );
          }
        } else if (xhr.status == 401) {
          completer.completeError(
            AuthenticationException('Invalid authentication token'),
          );
        } else if (xhr.status == 413) {
          try {
            final data = jsonDecode(xhr.responseText);
            completer.completeError(
              FileTooLargeException(data['error'] ?? 'File too large'),
            );
          } catch (e) {
            completer.completeError(FileTooLargeException('File too large'));
          }
        } else {
          try {
            final data = jsonDecode(xhr.responseText);
            completer.completeError(
              ApiException(
                data['error'] ?? 'Upload failed',
                statusCode: xhr.status,
              ),
            );
          } catch (e) {
            completer.completeError(
              ApiException('Upload failed', statusCode: xhr.status),
            );
          }
        }
      }).toJS,
    );

    xhr.addEventListener(
      'error',
      ((web.Event event) {
        completer.completeError(ApiException('Network error during upload'));
      }).toJS,
    );

    xhr.addEventListener(
      'abort',
      ((web.Event event) {
        completer.completeError(ApiException('Upload aborted'));
      }).toJS,
    );

    xhr.send(formData);

    return completer.future;
  }

  Future<DeleteResponse> deleteFile(String fileName) async {
    if (authToken == null || authToken!.isEmpty) {
      throw AuthenticationException('Authentication token required');
    }

    final response = await http.delete(
      Uri.parse('$baseUrl/api/delete/${Uri.encodeComponent(fileName)}'),
      headers: _headers,
    );

    if (response.statusCode == 200) {
      return DeleteResponse.fromJson(jsonDecode(response.body));
    } else if (response.statusCode == 401) {
      throw AuthenticationException('Invalid authentication token');
    } else if (response.statusCode == 404) {
      throw FileNotFoundException('File not found: $fileName');
    } else {
      final data = jsonDecode(response.body);
      throw ApiException(
        data['error'] ?? 'Delete failed',
        statusCode: response.statusCode,
      );
    }
  }

  /// Get file URL for viewing/serving
  String getFileUrl(String fileName) {
    return '$baseUrl/api/uploads/${Uri.encodeComponent(fileName)}';
  }
}

/// Base API exception
class ApiException implements Exception {
  ApiException(this.message, {this.statusCode});

  final String message;
  final int? statusCode;

  @override
  String toString() => 'ApiException: $message (status: $statusCode)';
}

/// Authentication exception
class AuthenticationException extends ApiException {
  AuthenticationException(super.message) : super(statusCode: 401);
}

/// File not found exception
class FileNotFoundException extends ApiException {
  FileNotFoundException(super.message) : super(statusCode: 404);
}

/// File too large exception
class FileTooLargeException extends ApiException {
  FileTooLargeException(super.message) : super(statusCode: 413);
}
