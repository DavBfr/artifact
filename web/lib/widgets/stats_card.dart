import 'package:jaspr/dom.dart';
import 'package:jaspr/jaspr.dart';

import '../bulma/bulma.dart';
import '../models/api_models.dart';
import '../utils/formatters.dart';

class StatsCard extends StatelessComponent {
  const StatsCard({required this.files, super.key});

  final List<FileInfo> files;

  @override
  Component build(BuildContext context) {
    final totalFiles = files.length;
    final totalSize = files.fold<int>(0, (sum, file) => sum + file.size);
    final lastUpload = files.isNotEmpty
        ? formatTimeAgo(files.first.modified)
        : 'Never';

    return BulmaLevel(classes: 'is-hidden-mobile', [
      BulmaLevelItem(
        heading: const Component.text('Total Files'),
        title: Component.text(totalFiles.toString()),
      ),
      BulmaLevelItem(
        heading: const Component.text('Total Size'),
        title: Component.text(formatBytes(totalSize)),
      ),
      BulmaLevelItem(
        heading: const Component.text('Last Upload'),
        title: Component.text(lastUpload),
      ),
    ]);
  }
}

class BulmaStat extends StatelessComponent {
  const BulmaStat({required this.title, required this.content, super.key});

  final Component title;
  final Component content;

  @override
  Component build(BuildContext context) {
    return div([
      div([title]),
      div([content]),
    ]);
  }
}
