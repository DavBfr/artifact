import 'package:jaspr/jaspr.dart';

import '../models/api_models.dart';
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

  List<FileInfo> get filteredFiles {
    if (query.isEmpty) return component.files;
    final q = query.toLowerCase();
    return component.files
        .where((f) => f.name.toLowerCase().contains(q))
        .toList();
  }

  String _getFileIcon(String fileName, String mimeType) {
    final extension = fileName.split('.').last.toLowerCase();

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
        extension == 'dart')
      return 'fas fa-file-code';

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

  @override
  Component build(BuildContext context) {
    final filesCount = component.files.isEmpty
        ? 'No files'
        : '${component.files.length} file${component.files.length != 1 ? 's' : ''}';

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
            span(classes: 'tag is-info is-light is-medium', id: 'files-count', [
              text(filesCount),
            ]),
          ]),
        ]),
      ]),

      // Files content
      if (component.files.isEmpty)
        // No files message - improved empty state
        div(id: 'no-files', classes: 'has-text-centered py-6', [
          div(classes: 'mb-5', [
            span(classes: 'icon is-large has-text-grey-lighter', [
              i(classes: 'fas fa-inbox fa-4x', []),
            ]),
          ]),
          p(classes: 'title is-4 has-text-grey-lighter mb-3', [
            text('No artifacts'),
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
                attributes: {'type': 'text', 'placeholder': 'Search'},
                events: events(
                  onInput: (String e) {
                    setState(() => query = e);
                  },
                ),
              ),
              span(classes: 'icon is-left', [
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
              attributes: {'style': 'cursor: default;'},
              [
                (file.mimeType.startsWith('image/') && file.size < 300 * 1024)
                    ? img(
                        src: file.url,
                        alt: file.name,
                        attributes: {
                          'style':
                              'width:48px;height:48px;object-fit:cover;border-radius:4px;',
                        },
                        classes: 'mr-3',
                      )
                    : span(
                        classes: 'panel-icon mr-4',
                        attributes: {
                          'style': 'font-size: 40px; margin-left:4px;',
                        },
                        [
                          i(
                            classes: _getFileIcon(file.name, file.mimeType),
                            [],
                          ),
                        ],
                      ),
                // File main column: name and small metadata stacked
                div([
                  div([text(file.name)]),
                  div(classes: 'is-size-7 has-text-grey', [
                    text(
                      '${formatTimeAgo(file.modified)} • ${formatBytes(file.size)}',
                    ),
                  ]),
                ]),
                // Actions aligned to the right
                div(classes: 'ml-auto', [
                  a(
                    href: file.url,
                    classes: 'button is-small is-primary is-light mr-2',
                    attributes: {'download': ''},
                    [
                      span(classes: 'icon', [
                        i(classes: 'fas fa-download', []),
                      ]),
                      span([text('Download')]),
                    ],
                  ),
                  if (component.isAuthenticated)
                    button(
                      classes: 'button is-small is-danger is-light',
                      onClick: () => component.onDelete(file.name),
                      [
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
