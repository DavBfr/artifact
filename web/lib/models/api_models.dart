import 'package:json_annotation/json_annotation.dart';

part 'api_models.g.dart';

/// File information model
@JsonSerializable()
class FileInfo {
  final String name;
  final int size;
  final String modified;
  final String url;

  FileInfo({
    required this.name,
    required this.size,
    required this.modified,
    required this.url,
  });

  factory FileInfo.fromJson(Map<String, dynamic> json) =>
      _$FileInfoFromJson(json);

  Map<String, dynamic> toJson() => _$FileInfoToJson(this);
}

/// Generic API response
@JsonSerializable(genericArgumentFactories: true)
class ApiResponse<T> {
  final bool success;
  final String? error;
  final String? message;
  final T? data;

  ApiResponse({required this.success, this.error, this.message, this.data});

  factory ApiResponse.fromJson(
    Map<String, dynamic> json,
    T Function(Object? json) fromJsonT,
  ) => _$ApiResponseFromJson(json, fromJsonT);

  Map<String, dynamic> toJson(Object? Function(T value) toJsonT) =>
      _$ApiResponseToJson(this, toJsonT);
}

/// List files response
@JsonSerializable()
class ListFilesResponse {
  final bool success;
  final List<FileInfo> files;
  final int count;
  final String? error;

  ListFilesResponse({
    required this.success,
    required this.files,
    required this.count,
    this.error,
  });

  factory ListFilesResponse.fromJson(Map<String, dynamic> json) =>
      _$ListFilesResponseFromJson(json);

  Map<String, dynamic> toJson() => _$ListFilesResponseToJson(this);
}

/// Upload file response
@JsonSerializable()
class UploadResponse {
  final bool success;
  final String? message;
  final FileInfo? file;
  final bool replaced;
  final String? error;

  UploadResponse({
    required this.success,
    this.message,
    this.file,
    required this.replaced,
    this.error,
  });

  factory UploadResponse.fromJson(Map<String, dynamic> json) =>
      _$UploadResponseFromJson(json);

  Map<String, dynamic> toJson() => _$UploadResponseToJson(this);
}

/// Config response
@JsonSerializable()
class ConfigResponse {
  final bool success;
  @JsonKey(name: 'max_content_length')
  final int maxContentLength;
  final String? error;

  ConfigResponse({
    required this.success,
    required this.maxContentLength,
    this.error,
  });

  factory ConfigResponse.fromJson(Map<String, dynamic> json) =>
      _$ConfigResponseFromJson(json);

  Map<String, dynamic> toJson() => _$ConfigResponseToJson(this);
}

/// Health check response
@JsonSerializable()
class HealthResponse {
  final String status;
  final String service;

  HealthResponse({required this.status, required this.service});

  factory HealthResponse.fromJson(Map<String, dynamic> json) =>
      _$HealthResponseFromJson(json);

  Map<String, dynamic> toJson() => _$HealthResponseToJson(this);
}

/// Delete file response
@JsonSerializable()
class DeleteResponse {
  final bool success;
  final String? message;
  final String? error;

  DeleteResponse({required this.success, this.message, this.error});

  factory DeleteResponse.fromJson(Map<String, dynamic> json) =>
      _$DeleteResponseFromJson(json);

  Map<String, dynamic> toJson() => _$DeleteResponseToJson(this);
}
