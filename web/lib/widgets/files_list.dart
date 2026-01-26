import 'package:jaspr/dom.dart';
import 'package:jaspr/jaspr.dart';

import '../models/api_models.dart';
import '../models/file_icon.dart';
import '../utils/formatters.dart';

class FilesList extends StatefulComponent {
  const FilesList({
    required this.files,
    required this.isAuthenticated,
    required this.onDelete,
    super.key,
  });

  final List<FileInfo> files;
  final bool isAuthenticated;
  final void Function(String) onDelete;

  @override
  State<FilesList> createState() => _FilesListState();
}

class _FilesListState extends State<FilesList> {
  String query = '';

  Iterable<FileInfo> get prefilteredFiles {
    return component.isAuthenticated
        ? component.files
        : component.files.where((e) => !e.name.startsWith('.'));
  }

  Iterable<FileInfo> get filteredFiles {
    if (query.isEmpty) return prefilteredFiles;
    final q = query.toLowerCase();
    return prefilteredFiles.where((f) => f.name.toLowerCase().contains(q));
  }

  @override
  Component build(BuildContext context) {
    final prefilteredCount = prefilteredFiles.length;

    final filesCount = prefilteredCount == 0
        ? 'No files'
        : '$prefilteredCount file${prefilteredCount != 1 ? 's' : ''}';

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
      if (prefilteredCount == 0)
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
          // Search block
          div(classes: 'panel-block', [
            p(classes: 'control has-icons-left', [
              input(
                classes: 'input',
                attributes: const {'type': 'text', 'placeholder': 'Search'},
                events: events(
                  onInput: (String e) {
                    setState(() => query = e);
                  },
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

          // File entries (filtered)
          for (final file in filteredFiles) ...[
            a(
              href: 'javascript:void(0);',
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
                      onClick: () => component.onDelete(file.name),
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
        ]),
    ]);
  }
}
