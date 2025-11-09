import 'package:jaspr/jaspr.dart';

import '../models/app_state.dart';
import '../utils/formatters.dart';

class StatsCard extends StatelessComponent {
  final List<FileInfo> files;

  const StatsCard({required this.files, super.key});

  @override
  Component build(BuildContext context) {
    final totalFiles = files.length;
    final totalSize = files.fold<int>(0, (sum, file) => sum + file.size);
    final lastUpload = files.isNotEmpty
        ? formatTimeAgo(files.first.modified)
        : 'Never';

    return div(classes: 'card stats-card', [
      div(classes: 'card-body', [
        div(classes: 'row text-center', [
          div(classes: 'col-md-4', [
            h4(id: 'total-files', [text(totalFiles.toString())]),
            p(classes: 'mb-0', [text('Total Files')]),
          ]),
          div(classes: 'col-md-4', [
            h4(id: 'total-size', [text(formatBytes(totalSize))]),
            p(classes: 'mb-0', [text('Total Size')]),
          ]),
          div(classes: 'col-md-4', [
            h4(id: 'last-upload', [text(lastUpload)]),
            p(classes: 'mb-0', [text('Last Upload')]),
          ]),
        ]),
      ]),
    ]);
  }
}
