import 'api_models.dart';

extension FileIcon on FileInfo {
  String get iconClass {
    final extension = name.split('.').last.toLowerCase();

    // Icon mapping based on file extension and mime type
    if (mimeType.startsWith('image/')) return 'fas fa-file-image';
    if (mimeType.startsWith('video/')) return 'fas fa-file-video';
    if (mimeType.startsWith('audio/')) return 'fas fa-file-audio';
    if (mimeType == 'application/pdf') return 'fas fa-file-pdf';
    if (mimeType.startsWith('text/') || extension == 'txt') {
      return 'fas fa-file-alt';
    }
    if (extension == 'doc' || extension == 'docx') return 'fas fa-file-word';
    if (extension == 'xls' || extension == 'xlsx') return 'fas fa-file-excel';
    if (extension == 'ppt' || extension == 'pptx') {
      return 'fas fa-file-powerpoint';
    }
    if (extension == 'zip' ||
        extension == 'rar' ||
        extension == '7z' ||
        extension == 'tbz' ||
        extension == 'tar' ||
        extension == 'bz2' ||
        extension == 'gz' ||
        extension == 'xz' ||
        extension == 'z' ||
        extension == 'deb' ||
        extension == 'rpm') {
      return 'fas fa-file-archive';
    }
    if (extension == 'json' ||
        extension == 'html' ||
        extension == 'htm' ||
        extension == 'css' ||
        extension == 'js' ||
        extension == 'dart') {
      return 'fas fa-file-code';
    }

    if (extension == 'exe' ||
        extension == 'bin' ||
        extension == 'dll' ||
        extension == 'so' ||
        extension == 'msi') {
      return 'fas fa-window-maximize';
    }
    if (extension == 'iso') {
      return 'fas fa-compact-disc';
    }

    // Default file icon
    return 'fas fa-file';
  }
}
