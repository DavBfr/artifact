import 'package:jaspr/jaspr.dart';

import '../models/api_models.dart';
import '../utils/formatters.dart';

class FilesList extends StatelessComponent {
  final List<FileInfo> files;
  final bool isAuthenticated;
  final void Function(String) onDelete;

  const FilesList({
    required this.files,
    required this.isAuthenticated,
    required this.onDelete,
    super.key,
  });

  @override
  Component build(BuildContext context) {
    final filesCount = files.isEmpty
        ? 'No files'
        : '${files.length} file${files.length != 1 ? 's' : ''}';

    return div(classes: 'has-background-white', [
      // Header with title and count
      nav(classes: 'level mb-4', [
        div(classes: 'level-left', [
          div(classes: 'level-item', [
            div([
              p(classes: 'title is-4 mb-1', [
                span(classes: 'icon-text', [
                  span(classes: 'icon has-text-primary', [
                    i(classes: 'fas fa-folder-open', []),
                  ]),
                  span([text('Artifacts')]),
                ]),
              ]),
            ]),
          ]),
        ]),
        div(classes: 'level-right', [
          div(classes: 'level-item', [
            span(classes: 'tag is-medium', id: 'files-count', [
              text(filesCount),
            ]),
          ]),
        ]),
      ]),

      div(classes: 'mb-4', [div(classes: 'is-divider', [])]),

      // Files content
      if (files.isEmpty)
        // No files message - improved empty state
        div(id: 'no-files', classes: 'has-text-centered py-6', [
          div(classes: 'mb-5', [
            span(classes: 'icon is-large has-text-grey-lighter', [
              i(classes: 'fas fa-inbox fa-4x', []),
            ]),
          ]),
          p(classes: 'title is-4 has-text-grey-dark mb-3', [
            text('No artifacts uploaded yet'),
          ]),
          p(classes: 'subtitle is-6 has-text-grey mb-4', [
            text('Upload your first artifact using the API'),
          ]),
          div(classes: 'content is-small has-text-grey', [
            p([
              text(
                'Use the authentication token to upload files via the API endpoint.',
              ),
            ]),
          ]),
        ])
      else
        // Files list with better spacing
        div(
          id: 'files-list',
          classes: 'files-container',
          files.map((file) => _buildFileItem(file)).toList(),
        ),
    ]);
  }

  Component _buildFileItem(FileInfo file) {
    return div(classes: 'mb-3', [
      div(classes: 'box is-shadowless has-background-light', [
        article(classes: 'media', [
          // File icon with better styling
          figure(classes: 'media-left', [
            span(
              classes:
                  'icon is-large has-text-white has-background-primary is-rounded p-4',
              [i(classes: 'fas fa-file-alt fa-2x', [])],
            ),
          ]),

          // File info with improved typography
          div(classes: 'media-content', [
            div(classes: 'content', [
              p(classes: 'mb-2', [
                strong(classes: 'is-size-5 has-text-dark', [text(file.name)]),
              ]),
              p(classes: 'mb-0', [
                span(classes: 'icon-text is-small has-text-grey', [
                  span(classes: 'icon', [i(classes: 'fas fa-clock', [])]),
                  span(classes: 'mr-3', [
                    text('${formatTimeAgo(file.modified)}'),
                  ]),
                ]),
                span(classes: 'icon-text is-small has-text-grey', [
                  span(classes: 'icon', [i(classes: 'fas fa-hdd', [])]),
                  span([text('${formatBytes(file.size)}')]),
                ]),
              ]),
            ]),
          ]),

          // Action buttons with better styling
          div(classes: 'media-right', [
            div(classes: 'buttons are-small', [
              a(
                href: file.url,
                classes: 'button is-primary is-light',
                attributes: {'download': ''},
                [
                  span(classes: 'icon', [i(classes: 'fas fa-download', [])]),
                  span([text('Download')]),
                ],
              ),
              if (isAuthenticated)
                button(
                  classes: 'button is-danger is-light',
                  onClick: () => onDelete(file.name),
                  [
                    span(classes: 'icon', [i(classes: 'fas fa-trash-alt', [])]),
                    span([text('Delete')]),
                  ],
                ),
            ]),
          ]),
        ]),
      ]),
    ]);
  }
}
