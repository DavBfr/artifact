import 'package:jaspr/jaspr.dart';

import '../models/api_models.dart';
import '../utils/formatters.dart';
import 'bulma_level.dart';

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

    return BulmaLevel([
      BulmaLevelItem(
        heading: text('Total Files'),
        title: text(totalFiles.toString()),
      ),
      BulmaLevelItem(
        heading: text('Total Size'),
        title: text(formatBytes(totalSize)),
      ),
      BulmaLevelItem(heading: text('Last Upload'), title: text(lastUpload)),
    ]);
  }
}

class BulmaStat extends StatelessComponent {
  final Component title;
  final Component content;

  const BulmaStat({required this.title, required this.content, super.key});

  @override
  Component build(BuildContext context) {
    return div([
      div([title]),
      div([content]),
    ]);
  }
}
