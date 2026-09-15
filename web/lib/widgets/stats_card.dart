import 'package:jaspr/dom.dart';
import 'package:jaspr/jaspr.dart';

import '../bulma/bulma.dart';
import '../models/api_models.dart';
import '../utils/formatters.dart';

class StatsCard extends StatelessComponent {
  const StatsCard({required this.stats, super.key});

  final StatsResponse? stats;

  @override
  Component build(BuildContext context) {
    final totalFiles = stats?.totalFiles ?? 0;
    final totalSize = stats?.totalSize ?? 0;
    final lastUpload = stats?.lastUpload;

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
        title: Component.text(
          lastUpload == null || lastUpload.isEmpty
              ? 'Never'
              : formatTimeAgo(lastUpload),
        ),
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
