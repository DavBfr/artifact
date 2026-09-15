import 'package:jaspr/dom.dart';
import 'package:jaspr/jaspr.dart';
import 'package:universal_web/web.dart' as web;

import '../bulma/bulma.dart';
import '../models/api_models.dart';
import '../models/file_icon.dart';
import '../utils/formatters.dart';

class FilesList extends StatefulComponent {
  const FilesList({
    required this.files,
    required this.isAuthenticated,
    required this.onDelete,
    required this.searchQuery,
    required this.onSearchChanged,
    required this.hasMore,
    required this.isLoadingMore,
    required this.onLoadMore,
    required this.filenameUrlsEnabled,
    super.key,
  });

  final List<FileInfo> files;
  final bool isAuthenticated;
  final void Function(FileInfo) onDelete;
  final String searchQuery;
  final void Function(String) onSearchChanged;
  final bool hasMore;
  final bool isLoadingMore;
  final VoidCallback onLoadMore;
  final bool filenameUrlsEnabled;

  @override
  State<FilesList> createState() => _FilesListState();
}

class _FilesListState extends State<FilesList> {
  void _copyShortLink(FileInfo file) {
    final shortLink = '${web.window.location.origin}${file.url}';
    web.window.navigator.clipboard.writeText(shortLink);
    NotificationMessenger.of(context).showNotification(
      BulmaNotification.success('Short link copied to clipboard'),
    );
  }

  void _copyFilenameLink(FileInfo file) {
    final filenameLink =
        '${web.window.location.origin}/f/${Uri.encodeComponent(file.name)}';
    web.window.navigator.clipboard.writeText(filenameLink);
    NotificationMessenger.of(context).showNotification(
      BulmaNotification.success('Filename link copied to clipboard'),
    );
  }

  @override
  Component build(BuildContext context) {
    final visibleCount = component.files.length;

    // The server no longer reports a grand total (pagination stops once a
    // page comes back empty), so this reflects files loaded so far.
    final filesCount = visibleCount == 0
        ? 'No files'
        : '$visibleCount file${visibleCount != 1 ? 's' : ''}${component.hasMore ? '+' : ''}';

    return div(classes: 'has-background-white', [
      // Header with title and count
      nav(classes: 'level mb-4', [
        const div(classes: 'level-left', [
          div(classes: 'level-item', [
            div([
              p(classes: 'title is-4 mb-1', [
                span(classes: 'icon-text', [
                  span(classes: 'icon has-text-primary', [
                    i(classes: 'fas fa-folder-open', []),
                  ]),
                  span([Component.text('Artifacts')]),
                ]),
              ]),
            ]),
          ]),
        ]),
        div(classes: 'level-right', [
          div(classes: 'level-item', [
            span(classes: 'tag is-info is-light is-medium', id: 'files-count', [
              Component.text(filesCount),
            ]),
          ]),
        ]),
      ]),

      // Files content
      if (visibleCount == 0 && component.searchQuery.isEmpty)
        // No files message - improved empty state
        const div(id: 'no-files', classes: 'has-text-centered py-6', [
          div(classes: 'mb-5', [
            span(classes: 'icon is-large has-text-grey-lighter', [
              i(classes: 'fas fa-inbox fa-4x', []),
            ]),
          ]),
          p(classes: 'title is-4 has-text-grey-lighter mb-3', [
            Component.text('No artifacts'),
          ]),
        ])
      else
        // Bulma panel-style files list (panel heading, search, and panel-blocks)
        nav(classes: 'panel is-shadowless', [
          // Search block (server-side filter, debounced by the parent)
          div(classes: 'panel-block', [
            p(classes: 'control has-icons-left', [
              input(
                classes: 'input',
                attributes: {
                  'type': 'text',
                  'placeholder': 'Search',
                  'value': component.searchQuery,
                },
                events: events(
                  onInput: (String e) => component.onSearchChanged(e),
                ),
              ),
              const span(classes: 'icon is-left', [
                i(
                  classes: 'fas fa-search',
                  attributes: {'aria-hidden': 'true'},
                  [],
                ),
              ]),
            ]),
          ]),

          // File entries (current page)
          for (final file in component.files) ...[
            div(
              classes: 'panel-block',
              attributes: const {'style': 'cursor: default;'},
              [
                (file.mimeType.startsWith('image/') && file.size < 500 * 1024)
                    ? img(
                        src: file.url,
                        alt: '',
                        attributes: const {
                          'style':
                              'width:48px;height:48px;object-fit:cover;border-radius:4px;',
                        },
                        classes: 'mr-3',
                      )
                    : span(
                        classes: 'panel-icon mr-4',
                        attributes: const {
                          'style': 'font-size: 40px; margin-left:4px;',
                        },
                        [i(classes: file.iconClass, const [])],
                      ),
                // File main column: name and small metadata stacked
                div([
                  div([Component.text(file.name)]),
                  div(classes: 'is-size-7 has-text-grey', [
                    Component.text(
                      '${formatTimeAgo(file.modified)} • ${formatBytes(file.size)}',
                    ),
                  ]),
                ]),
                // Actions aligned to the right
                div(classes: 'ml-auto', [
                  button(
                    classes: 'button is-small is-link is-light mr-2',
                    attributes: const {'title': 'Copy short link'},
                    onClick: () => _copyShortLink(file),
                    const [
                      span(classes: 'icon', [i(classes: 'fas fa-link', [])]),
                    ],
                  ),
                  if (component.filenameUrlsEnabled)
                    button(
                      classes: 'button is-small is-info is-light mr-2',
                      attributes: const {'title': 'Copy filename link'},
                      onClick: () => _copyFilenameLink(file),
                      const [
                        span(classes: 'icon', [
                          i(classes: 'fas fa-file-signature', []),
                        ]),
                      ],
                    ),
                  a(
                    href: file.url,
                    classes: 'button is-small is-primary is-light mr-2',
                    attributes: const {'download': ''},
                    const [
                      span(classes: 'icon', [
                        i(classes: 'fas fa-download', []),
                      ]),
                      span([Component.text('Download')]),
                    ],
                  ),
                  if (component.isAuthenticated)
                    button(
                      classes: 'button is-small is-danger is-light',
                      onClick: () => component.onDelete(file),
                      const [
                        span(classes: 'icon', [
                          i(classes: 'fas fa-trash-alt', []),
                        ]),
                      ],
                    ),
                ]),
              ],
            ),
          ],

          if (component.hasMore)
            div(classes: 'panel-block is-justify-content-center', [
              button(
                classes:
                    'button is-small is-light'
                    '${component.isLoadingMore ? ' is-loading' : ''}',
                onClick: component.isLoadingMore ? () {} : component.onLoadMore,
                const [Component.text('Load more')],
              ),
            ]),
        ]),
    ]);
  }
}
