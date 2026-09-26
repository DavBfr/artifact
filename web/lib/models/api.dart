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

  /// List files, with server-side pagination and search filtering. Always
  /// ordered newest-first, matching the UI's fixed sort policy.
  Future<ListFilesResponse> listFiles({
    int limit = 50,
    int offset = 0,
    String search = '',
  }) async {
    final uri = Uri.parse('$baseUrl/api/files').replace(
      queryParameters: {
        'limit': '$limit',
        'offset': '$offset',
        'order': '-date',
        if (search.isNotEmpty) 'search': search,
      },
    );
    final response = await http.get(uri, headers: _headers);

    if (response.statusCode == 200) {
      return ListFilesResponse.fromJson(jsonDecode(response.body));
    } else if (response.statusCode == 401) {
      throw AuthenticationException('Authentication required to list files');
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

  /// Get aggregate file stats (total files, total size, last upload)
  Future<StatsResponse> getStats() async {
    final response = await http.get(
      Uri.parse('$baseUrl/api/stats'),
      headers: _headers,
    );

    if (response.statusCode == 200) {
      return StatsResponse.fromJson(jsonDecode(response.body));
    } else if (response.statusCode == 401) {
      throw AuthenticationException('Authentication required to get stats');
    } else {
      throw ApiException(
        'Failed to get stats',
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

  /// Delete a file by its short link slug (the id in `/s/{slug}`)
  Future<DeleteResponse> deleteFile(String slug) async {
    if (authToken == null || authToken!.isEmpty) {
      throw AuthenticationException('Authentication token required');
    }

    final response = await http.delete(
      Uri.parse('$baseUrl/api/delete/${Uri.encodeComponent(slug)}'),
      headers: _headers,
    );

    if (response.statusCode == 200) {
      return DeleteResponse.fromJson(jsonDecode(response.body));
    } else if (response.statusCode == 401) {
      throw AuthenticationException('Invalid authentication token');
    } else if (response.statusCode == 404) {
      throw FileNotFoundException('File not found: $slug');
    } else {
      final data = jsonDecode(response.body);
      throw ApiException(
        data['error'] ?? 'Delete failed',
        statusCode: response.statusCode,
      );
    }
  }

  /// List the tags attached to a file, by short link slug.
  Future<FileTagsResponse> listTags(String slug) async {
    final response = await http.get(
      Uri.parse('$baseUrl/api/tags/${Uri.encodeComponent(slug)}'),
      headers: _headers,
    );
    return _parseTagsResponse(response, 'list tags');
  }

  /// Attach tags to a file, by short link slug. A tag that already belongs to
  /// another file is *moved* to this one rather than rejected - the response
  /// reports each move in `moved`.
  Future<FileTagsResponse> addTags(String slug, List<String> tags) async {
    if (authToken == null || authToken!.isEmpty) {
      throw AuthenticationException('Authentication token required');
    }

    final response = await http.post(
      Uri.parse('$baseUrl/api/tags/${Uri.encodeComponent(slug)}'),
      headers: _headers,
      body: jsonEncode({'tags': tags}),
    );
    return _parseTagsResponse(response, 'add tags');
  }

  /// Detach a single tag from a file, by short link slug. The tag itself is
  /// free for another file afterwards.
  Future<FileTagsResponse> removeTag(String slug, String tag) async {
    if (authToken == null || authToken!.isEmpty) {
      throw AuthenticationException('Authentication token required');
    }

    final response = await http.delete(
      Uri.parse(
        '$baseUrl/api/tags/${Uri.encodeComponent(slug)}/${Uri.encodeComponent(tag)}',
      ),
      headers: _headers,
    );
    return _parseTagsResponse(response, 'remove tag');
  }

  /// Page size used when looking a tag up in the global listing; the server
  /// caps it at ART_MAX_LIST_LIMIT regardless.
  static const int _tagLookupLimit = 500;

  /// Returns the file that currently owns [tag], or null when no live file does.
  /// A tag resolves to exactly one file, so callers use this to warn before a
  /// tag is moved off another file. A malformed tag returns null too - the add
  /// call itself is what reports the real error for one.
  Future<TagInfo?> findTagOwner(String tag) async {
    final canonical = _canonicalTag(tag);
    if (canonical == null) return null;

    // The listing's `search` is a substring match, so search by the name and
    // then match the whole tag exactly.
    final name = canonical.substring(0, canonical.lastIndexOf(':'));
    final response = await http.get(
      Uri.parse(
        '$baseUrl/api/tags',
      ).replace(queryParameters: {'search': name, 'limit': '$_tagLookupLimit'}),
      headers: _headers,
    );

    if (response.statusCode != 200) {
      throw ApiException(
        'Failed to look up tag "$canonical"',
        statusCode: response.statusCode,
      );
    }

    final listing = TagListResponse.fromJson(jsonDecode(response.body));
    for (final info in listing.tags) {
      if (info.tag == canonical) return info;
    }
    return null;
  }

  /// Splits a tag into the canonical `name:suffix` form the server stores, so an
  /// exact lookup is possible. A bare name means `:latest`, and anything the
  /// server would reject (an empty name or suffix) yields null here as well.
  static String? _canonicalTag(String value) {
    final trimmed = value.trim();
    if (trimmed.isEmpty) return null;

    // The server splits on the LAST colon, so mirror that here.
    final separator = trimmed.lastIndexOf(':');
    if (separator < 0) return '$trimmed:latest';

    final name = trimmed.substring(0, separator);
    final suffix = trimmed.substring(separator + 1);
    if (name.isEmpty || suffix.isEmpty) return null;
    return '$name:$suffix';
  }

  /// Shared handling for the tag endpoints: the message on failure is the
  /// server's, which is far more useful than a generic one (it names the
  /// offending tag, for instance).
  FileTagsResponse _parseTagsResponse(http.Response response, String action) {
    if (response.statusCode == 200) {
      return FileTagsResponse.fromJson(jsonDecode(response.body));
    }
    if (response.statusCode == 401) {
      throw AuthenticationException('Invalid authentication token');
    }

    String? message;
    try {
      message =
          (jsonDecode(response.body) as Map<String, dynamic>)['error']
              as String?;
    } catch (_) {
      message = null;
    }
    throw ApiException(
      message ?? 'Failed to $action',
      statusCode: response.statusCode,
    );
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
