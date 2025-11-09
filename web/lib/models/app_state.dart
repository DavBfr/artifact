import 'api_models.dart';

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
