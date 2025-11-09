String formatBytes(int bytes) {
  if (bytes == 0) return '0 B';
  const sizes = ['B', 'KB', 'MB', 'GB'];
  final i = (bytes.bitLength - 1) ~/ 10;
  final size = bytes / (1 << (i * 10));
  return '${size.toStringAsFixed(1)} ${sizes[i]}';
}

String formatTimeAgo(String isoString) {
  final date = DateTime.parse(isoString);
  final now = DateTime.now();
  final difference = now.difference(date);
  final seconds = difference.inSeconds;

  if (seconds < 60) return 'just now';
  if (seconds < 3600) {
    final minutes = seconds ~/ 60;
    return '$minutes minute${minutes != 1 ? 's' : ''} ago';
  }
  if (seconds < 86400) {
    final hours = seconds ~/ 3600;
    return '$hours hour${hours != 1 ? 's' : ''} ago';
  }
  if (seconds < 2592000) {
    final days = seconds ~/ 86400;
    return '$days day${days != 1 ? 's' : ''} ago';
  }
  if (seconds < 31536000) {
    final months = seconds ~/ 2592000;
    return '$months month${months != 1 ? 's' : ''} ago';
  }
  final years = seconds ~/ 31536000;
  return '$years year${years != 1 ? 's' : ''} ago';
}
