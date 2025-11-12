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
    required String url,
  }) : _url = url;

  factory FileInfo.fromJson(Map<String, dynamic> json) =>
      _$FileInfoFromJson(json);

  final String name;
  final int size;
  final String modified;
  final String mimeType;
  final String _url;

  String get url => 'http://localhost:9080/$_url';

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
@JsonSerializable()
class ConfigResponse {
  ConfigResponse({
    required this.success,
    required this.maxContentLength,
    this.error,
  });

  factory ConfigResponse.fromJson(Map<String, dynamic> json) =>
      _$ConfigResponseFromJson(json);

  final bool success;
  @JsonKey(name: 'max_content_length')
  final int maxContentLength;
  final String? error;

  Map<String, dynamic> toJson() => _$ConfigResponseToJson(this);
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
