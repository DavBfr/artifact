import 'package:jaspr/jaspr.dart';

import '../models/app_state.dart';
import '../utils/formatters.dart';

class FilesList extends StatelessComponent {
  final List<FileInfo> files;
  final bool isLoading;
  final bool isAuthenticated;
  final void Function(String) onDelete;

  const FilesList({
    required this.files,
    required this.isLoading,
    required this.isAuthenticated,
    required this.onDelete,
    super.key,
  });

  @override
  Component build(BuildContext context) {
    final filesCount = files.isEmpty
        ? 'No files'
        : '${files.length} file${files.length != 1 ? 's' : ''}';

    return div(classes: 'row', [
      div(classes: 'col-12', [
        div(classes: 'card', [
          div(
            classes:
                'card-header d-flex justify-content-between align-items-center',
            [
              h5(classes: 'card-title mb-0', [
                i(classes: 'bi bi-files', []),
                text(' Artifacts'),
              ]),
              small(classes: 'text-muted', id: 'files-count', [
                text(filesCount),
              ]),
            ],
          ),
          div(classes: 'card-body p-0', [
            // Loading spinner
            if (isLoading)
              div(id: 'loading', classes: 'text-center p-4', [
                div(
                  classes: 'spinner-border text-primary',
                  attributes: {'role': 'status'},
                  [
                    span(classes: 'visually-hidden', [text('Loading...')]),
                  ],
                ),
                p(classes: 'mt-2 text-muted', [text('Loading artifacts...')]),
              ])
            else if (files.isEmpty)
              // No files message
              div(id: 'no-files', classes: 'text-center p-5', [
                i(classes: 'bi bi-inbox fs-1 text-muted', []),
                h5(classes: 'mt-3 text-muted', [
                  text('No artifacts uploaded yet'),
                ]),
                p(classes: 'text-muted', [
                  text(
                    'Use the API with authentication token to upload artifacts.',
                  ),
                ]),
              ])
            else
              // Files list
              div(
                id: 'files-list',
                files.map((file) => _buildFileItem(file)).toList(),
              ),
          ]),
        ]),
      ]),
    ]);
  }

  Component _buildFileItem(FileInfo file) {
    return div(classes: 'file-item border-bottom p-3', [
      div(classes: 'd-flex align-items-center', [
        div(classes: 'file-icon bg-primary text-white', [
          i(classes: 'bi bi-file-earmark', []),
        ]),
        div(classes: 'flex-grow-1', [
          h6(classes: 'mb-1', [text(file.name)]),
          small(classes: 'text-muted', [
            i(classes: 'bi bi-calendar', []),
            text(' ${formatTimeAgo(file.modified)} • '),
            i(classes: 'bi bi-hdd', []),
            text(' ${formatBytes(file.size)}'),
          ]),
        ]),
        div(classes: 'ms-3', [
          a(
            href: file.url,
            classes: 'btn btn-outline-primary btn-sm',
            attributes: {'download': ''},
            [i(classes: 'bi bi-download', []), text(' Download')],
          ),
          if (isAuthenticated)
            button(
              classes: 'btn btn-outline-danger btn-sm ms-2',
              events: {'click': (_) => onDelete(file.name)},
              [i(classes: 'bi bi-trash', []), text(' Delete')],
            ),
        ]),
      ]),
    ]);
  }
}
