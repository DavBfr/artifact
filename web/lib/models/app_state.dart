class AppState {
  final List<FileInfo> files;
  final String? authToken;
  final int maxContentLength;
  final bool isLoading;
  final bool isUploading;
  final String? uploadingFileName;
  final int uploadProgress;

  const AppState({
    this.files = const [],
    this.authToken,
    this.maxContentLength = 100 * 1024 * 1024, // 100MB default
    this.isLoading = false,
    this.isUploading = false,
    this.uploadingFileName,
    this.uploadProgress = 0,
  });

  AppState copyWith({
    List<FileInfo>? files,
    String? authToken,
    bool clearAuthToken = false,
    int? maxContentLength,
    bool? isLoading,
    bool? isUploading,
    String? uploadingFileName,
    int? uploadProgress,
  }) {
    return AppState(
      files: files ?? this.files,
      authToken: clearAuthToken ? null : (authToken ?? this.authToken),
      maxContentLength: maxContentLength ?? this.maxContentLength,
      isLoading: isLoading ?? this.isLoading,
      isUploading: isUploading ?? this.isUploading,
      uploadingFileName: uploadingFileName ?? this.uploadingFileName,
      uploadProgress: uploadProgress ?? this.uploadProgress,
    );
  }
}

class FileInfo {
  final String name;
  final int size;
  final String modified;
  final String url;

  const FileInfo({
    required this.name,
    required this.size,
    required this.modified,
    required this.url,
  });

  factory FileInfo.fromJson(Map<String, dynamic> json) {
    return FileInfo(
      name: json['name'] as String,
      size: json['size'] as int,
      modified: json['modified'] as String,
      url: json['url'] as String,
    );
  }
}
