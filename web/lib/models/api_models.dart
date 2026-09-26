import 'dart:math';

import 'package:json_annotation/json_annotation.dart';

part 'api_models.g.dart';

/// File information model
@JsonSerializable(fieldRename: FieldRename.snake)
class FileInfo {
  FileInfo({
    required this.name,
    required this.size,
    required this.modified,
    required this.mimeType,
    required this.url,
    this.tags = const <String>[],
  });

  factory FileInfo.fromJson(Map<String, dynamic> json) =>
      _$FileInfoFromJson(json);

  final String name;
  final int size;
  final String modified;
  final String mimeType;
  final String url;

  /// Tags attached to this file, each in `name:suffix` form (e.g. `cat:latest`).
  /// The server always sends an array, but this defaults to empty so a response
  /// from an older server still parses.
  @JsonKey(defaultValue: <String>[])
  final List<String> tags;

  // The short link slug (the last path segment of `url`, e.g. "/s/aB3xQ").
  String get slug => url.split('/').last;

  Map<String, dynamic> toJson() => _$FileInfoToJson(this);
}

/// Generic API response
@JsonSerializable(genericArgumentFactories: true)
class ApiResponse<T> {
  ApiResponse({required this.success, this.error, this.message, this.data});

  factory ApiResponse.fromJson(
    Map<String, dynamic> json,
    T Function(Object? json) fromJsonT,
  ) => _$ApiResponseFromJson(json, fromJsonT);

  final bool success;
  final String? error;
  final String? message;
  final T? data;

  Map<String, dynamic> toJson(Object? Function(T value) toJsonT) =>
      _$ApiResponseToJson(this, toJsonT);
}

/// List files response
@JsonSerializable()
class ListFilesResponse {
  ListFilesResponse({
    required this.success,
    required this.files,
    required this.count,
    this.error,
  });

  factory ListFilesResponse.fromJson(Map<String, dynamic> json) =>
      _$ListFilesResponseFromJson(json);

  final bool success;
  final List<FileInfo> files;
  final int count;
  final String? error;

  Map<String, dynamic> toJson() => _$ListFilesResponseToJson(this);
}

/// Upload file response
@JsonSerializable()
class UploadResponse {
  UploadResponse({
    required this.success,
    this.message,
    this.file,
    required this.replaced,
    this.error,
  });

  factory UploadResponse.fromJson(Map<String, dynamic> json) =>
      _$UploadResponseFromJson(json);

  final bool success;
  final String? message;
  final FileInfo? file;
  final bool replaced;
  final String? error;

  Map<String, dynamic> toJson() => _$UploadResponseToJson(this);
}

/// Config response
@JsonSerializable(fieldRename: FieldRename.snake)
class ConfigResponse {
  const ConfigResponse({
    required this.success,
    this.filenameUrlsEnabled = false,
    this.maxContentLength = 100 * 1024 * 1024,
    this.maxListLimit = 500,
    this.error,
  });

  factory ConfigResponse.fromJson(Map<String, dynamic> json) =>
      _$ConfigResponseFromJson(json);

  static const empty = ConfigResponse(success: true);

  final bool success;
  final int maxContentLength;
  final int maxListLimit;
  final bool filenameUrlsEnabled;
  final String? error;

  int get pageSize => min(maxListLimit, 50);

  Map<String, dynamic> toJson() => _$ConfigResponseToJson(this);
}

/// Stats response
@JsonSerializable(fieldRename: FieldRename.snake)
class StatsResponse {
  StatsResponse({
    required this.success,
    required this.totalFiles,
    required this.totalSize,
    this.lastUpload,
    this.error,
  });

  factory StatsResponse.fromJson(Map<String, dynamic> json) =>
      _$StatsResponseFromJson(json);

  final bool success;
  final int totalFiles;
  final int totalSize;
  final String? lastUpload;
  final String? error;

  Map<String, dynamic> toJson() => _$StatsResponseToJson(this);
}

/// Health check response
@JsonSerializable()
class HealthResponse {
  HealthResponse({required this.status, required this.service});

  factory HealthResponse.fromJson(Map<String, dynamic> json) =>
      _$HealthResponseFromJson(json);
  final String status;
  final String service;

  Map<String, dynamic> toJson() => _$HealthResponseToJson(this);
}

/// Delete file response
@JsonSerializable()
class DeleteResponse {
  DeleteResponse({required this.success, this.message, this.error});

  factory DeleteResponse.fromJson(Map<String, dynamic> json) =>
      _$DeleteResponseFromJson(json);

  final bool success;
  final String? message;
  final String? error;

  Map<String, dynamic> toJson() => _$DeleteResponseToJson(this);
}

/// Response of the tag endpoints: the file's full tag list *after* the
/// operation, plus which tags were re-pointed from another file.
@JsonSerializable()
class FileTagsResponse {
  FileTagsResponse({
    required this.success,
    this.slug = '',
    this.tags = const <String>[],
    this.moved,
    this.message,
    this.error,
  });

  factory FileTagsResponse.fromJson(Map<String, dynamic> json) =>
      _$FileTagsResponseFromJson(json);

  final bool success;

  @JsonKey(defaultValue: '')
  final String slug;

  @JsonKey(defaultValue: <String>[])
  final List<String> tags;

  /// Tags that were moved away from another file, keyed by tag with the
  /// previous file's slug as the value (e.g. `{"cat:latest": "aB3xQ"}`).
  final Map<String, String>? moved;
  final String? message;
  final String? error;

  Map<String, dynamic> toJson() => _$FileTagsResponseToJson(this);
}

/// A tag and the file it currently points at, as returned by `GET /api/tags`.
@JsonSerializable(fieldRename: FieldRename.snake)
class TagInfo {
  TagInfo({
    required this.name,
    required this.suffix,
    required this.tag,
    required this.slug,
    required this.url,
    this.fileName = '',
    this.created = '',
  });

  factory TagInfo.fromJson(Map<String, dynamic> json) =>
      _$TagInfoFromJson(json);

  final String name;
  final String suffix;

  /// The canonical `name:suffix` form.
  final String tag;

  /// The slug of the file this tag resolves to.
  final String slug;

  /// The `/t/{tag}` download path.
  final String url;

  /// Display name of the file the tag resolves to.
  @JsonKey(defaultValue: '')
  final String fileName;
  final String created;

  Map<String, dynamic> toJson() => _$TagInfoToJson(this);
}

/// Response of `GET /api/tags`.
@JsonSerializable()
class TagListResponse {
  TagListResponse({
    required this.success,
    this.tags = const <TagInfo>[],
    this.count = 0,
    this.error,
  });

  factory TagListResponse.fromJson(Map<String, dynamic> json) =>
      _$TagListResponseFromJson(json);

  final bool success;

  @JsonKey(defaultValue: <TagInfo>[])
  final List<TagInfo> tags;
  final int count;
  final String? error;

  Map<String, dynamic> toJson() => _$TagListResponseToJson(this);
}
