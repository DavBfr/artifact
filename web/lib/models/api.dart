import 'dart:convert';

import 'package:http/http.dart' as http;
import 'package:jaspr/jaspr.dart';

import 'api_models.dart';

/// API client for communicating with the artifact server
class ArtifactApiClient {
  ArtifactApiClient({required this.baseUrl, this.authToken});

  factory ArtifactApiClient.base({String? authToken}) {
    final baseUrl = kDebugMode ? 'http://127.0.0.1:9080' : Uri.base.toString();
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

  /// Upload a file
  Future<UploadResponse> uploadFile({
    required List<int> fileBytes,
    required String fileName,
  }) async {
    if (authToken == null || authToken!.isEmpty) {
      throw AuthenticationException('Authentication token required');
    }

    final request = http.MultipartRequest(
      'POST',
      Uri.parse('$baseUrl/api/upload'),
    );

    request.headers['Authorization'] = 'Bearer $authToken';
    request.files.add(
      http.MultipartFile.fromBytes('file', fileBytes, filename: fileName),
    );

    final streamedResponse = await request.send();
    final response = await http.Response.fromStream(streamedResponse);

    if (response.statusCode == 201 || response.statusCode == 200) {
      return UploadResponse.fromJson(jsonDecode(response.body));
    } else if (response.statusCode == 401) {
      throw AuthenticationException('Invalid authentication token');
    } else if (response.statusCode == 413) {
      final data = jsonDecode(response.body);
      throw FileTooLargeException(data['error'] ?? 'File too large');
    } else {
      final data = jsonDecode(response.body);
      throw ApiException(
        data['error'] ?? 'Upload failed',
        statusCode: response.statusCode,
      );
    }
  }

  /// Delete a file
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

  /// Get download URL for a file
  String getDownloadUrl(String fileName) {
    return '$baseUrl/api/download/${Uri.encodeComponent(fileName)}';
  }

  /// Get file URL for viewing/serving
  String getFileUrl(String fileName) {
    return '$baseUrl/api/uploads/${Uri.encodeComponent(fileName)}';
  }
}

/// Base API exception
class ApiException implements Exception {
  final String message;
  final int? statusCode;

  ApiException(this.message, {this.statusCode});

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
