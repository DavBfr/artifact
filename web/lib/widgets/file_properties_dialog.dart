import 'package:jaspr/dom.dart';
import 'package:jaspr/jaspr.dart';
import 'package:universal_web/web.dart' as web;

import '../bulma/bulma.dart';
import '../models/api.dart';
import '../models/api_models.dart';
import '../utils/formatters.dart';

/// Everything the API knows about a single file, opened by clicking its row in
/// the list. The tag mutations live here too, since this is where a user can
/// see what a file is already tagged as.
class FilePropertiesDialog extends StatefulComponent {
  const FilePropertiesDialog({
    required this.file,
    required this.api,
    required this.filenameUrlsEnabled,
    required this.onChanged,
    required this.onDelete,
    required this.onClose,
    super.key,
  });

  final FileInfo file;
  final ArtifactApiClient api;
  final bool filenameUrlsEnabled;

  /// Called after a successful tag mutation so the list behind the dialog can
  /// pick up the change (a tag add can also remove it from another file).
  final VoidCallback onChanged;

  /// Deletes the file, confirmation included.
  final Future<void> Function() onDelete;

  final VoidCallback onClose;

  @override
  State<FilePropertiesDialog> createState() => _FilePropertiesDialogState();
}

class _FilePropertiesDialogState extends State<FilePropertiesDialog> {
  /// Mirrors the server's list, refreshed from every tag response so the
  /// dialog never shows a stale tag set.
  late List<String> _tags = List<String>.of(component.file.tags);
  String _newTag = '';
  bool _isBusy = false;

  String get _slug => component.file.slug;

  String _absoluteUrl(String path) => '${web.window.location.origin}$path';

  void _notify(Component notification) {
    NotificationMessenger.of(context).showNotification(notification);
  }

  void _copy(String value, String label) {
    web.window.navigator.clipboard.writeText(value);
    _notify(BulmaNotification.success('$label copied to clipboard'));
  }

  Future<void> _addTag() async {
    final tag = _newTag.trim();
    if (tag.isEmpty || _isBusy) return;

    setState(() => _isBusy = true);
    try {
      // A tag resolves to exactly one file, so adding one that another file
      // owns takes it away from there - confirm that before doing it.
      TagInfo? owner;
      try {
        owner = await component.api.findTagOwner(tag);
      } on ApiException {
        // Only a courtesy check: if the lookup fails, let the add call decide
        // (its response reports any move in `moved`).
        owner = null;
      }

      if (owner != null && owner.slug != _slug) {
        setState(() => _isBusy = false);
        final confirmed = await _confirm(
          title: 'Move tag',
          message:
              'The tag "${owner.tag}" belongs to "${owner.fileName}". '
              'Moving it here removes it from that file.',
          confirmLabel: 'Move tag',
        );
        if (!confirmed) return;
        setState(() => _isBusy = true);
      }

      final response = await component.api.addTags(_slug, [tag]);
      setState(() {
        _tags = response.tags;
        _newTag = '';
        _isBusy = false;
      });
      component.onChanged();

      final moved = response.moved;
      _notify(
        moved == null || moved.isEmpty
            ? BulmaNotification.success('Tag "$tag" added')
            : BulmaNotification.success(
                'Tag "$tag" moved from file ${moved.values.first}',
              ),
      );
    } on ApiException catch (e) {
      setState(() => _isBusy = false);
      _notify(BulmaNotification.error(e.message, title: 'Could not add tag'));
    }
  }

  Future<void> _removeTag(String tag) async {
    if (_isBusy) return;

    final confirmed = await _confirm(
      title: 'Remove tag',
      message: 'Remove the tag "$tag" from "${component.file.name}"?',
      confirmLabel: 'Remove',
    );
    if (!confirmed) return;

    setState(() => _isBusy = true);
    try {
      final response = await component.api.removeTag(_slug, tag);
      setState(() {
        _tags = response.tags;
        _isBusy = false;
      });
      component.onChanged();
      _notify(BulmaNotification.success('Tag "$tag" removed'));
    } on ApiException catch (e) {
      setState(() => _isBusy = false);
      _notify(
        BulmaNotification.error(e.message, title: 'Could not remove tag'),
      );
    }
  }

  Future<void> _delete() async {
    await component.onDelete();
    component.onClose();
  }

  /// Asks the user to confirm a change in a dialog stacked above this one, and
  /// only reports true when they explicitly confirm it.
  Future<bool> _confirm({
    required String title,
    required String message,
    required String confirmLabel,
    BulmaColor confirmColor = BulmaColor.danger,
  }) async {
    final result = await DialogManager.of(context).showDialog<bool>(
      (onComplete) => AlertDialog(
        title: Component.text(title),
        content: [Component.text(message)],
        actions: [
          BulmaButton(
            color: confirmColor,
            onPressed: () => onComplete(true),
            child: Component.text(confirmLabel),
          ),
          BulmaButton(
            onPressed: () => onComplete(),
            child: const Component.text('Cancel'),
          ),
        ],
      ),
    );
    return result == true;
  }

  @override
  Component build(BuildContext context) {
    final file = component.file;

    return AlertDialog(
      title: const span(classes: 'icon-text', [
        span(classes: 'icon has-text-primary', [
          i(classes: 'fas fa-circle-info', []),
        ]),
        span([Component.text('File properties')]),
      ]),
      content: [
        _row(
          'Name',
          span(classes: 'has-text-weight-semibold', [
            Component.text(file.name),
          ]),
        ),
        _row(
          'Size',
          span(
            attributes: {'title': '${file.size} bytes'},
            [Component.text(formatBytes(file.size))],
          ),
        ),
        _row(
          'Uploaded',
          span(
            attributes: {'title': file.modified},
            [
              Component.text(
                '${formatDateTime(file.modified)} (${formatTimeAgo(file.modified)})',
              ),
            ],
          ),
        ),
        _row(
          'Type',
          span([
            Component.text(file.mimeType.isEmpty ? 'unknown' : file.mimeType),
          ]),
        ),
        _row(
          'Slug',
          span(classes: 'is-family-monospace', [Component.text(_slug)]),
        ),
        _row(
          'Short link',
          div(classes: 'is-flex is-align-items-center', [
            a(
              href: file.url,
              attributes: {'title': _absoluteUrl(file.url)},
              [Component.text(_absoluteUrl(file.url))],
            ),
            _copyButton(
              title: 'Copy short link',
              onPressed: () => _copy(_absoluteUrl(file.url), 'Short link'),
            ),
          ]),
        ),
        if (component.filenameUrlsEnabled)
          _row(
            'Filename link',
            div(classes: 'is-flex is-align-items-center', [
              a(
                href: '/f/${Uri.encodeComponent(file.name)}',
                attributes: {
                  'title': _absoluteUrl('/f/${Uri.encodeComponent(file.name)}'),
                },
                [
                  Component.text(
                    '${web.window.location.origin}/f/${Uri.encodeComponent(file.name)}',
                  ),
                ],
              ),
              _copyButton(
                title: 'Copy filename link',
                onPressed: () => _copy(
                  _absoluteUrl('/f/${Uri.encodeComponent(file.name)}'),
                  'Filename link',
                ),
              ),
            ]),
          ),

        const hr(classes: 'my-3'),

        div(classes: 'is-flex is-justify-content-space-between mb-2', [
          span(classes: 'has-text-grey is-size-7', [
            Component.text(_tags.isEmpty ? 'Tags' : 'Tags (${_tags.length})'),
          ]),
          if (_tags.isNotEmpty)
            const span(classes: 'has-text-grey is-size-7', [
              Component.text('click a tag to copy its link'),
            ]),
        ]),

        if (_tags.isEmpty)
          const p(classes: 'has-text-grey is-size-7 mb-3', [
            Component.text('No tags on this file yet.'),
          ])
        else
          div(classes: 'tags mb-3', [for (final tag in _tags) _tagChip(tag)]),

        if (component.api.canAddTags)
          div(classes: 'field has-addons mb-0', [
            div(classes: 'control is-expanded', [
              input(
                classes: 'input is-small',
                attributes: {
                  'type': 'text',
                  'placeholder': 'name:suffix (e.g. cat:latest)',
                  'value': _newTag,
                  'autocomplete': 'off',
                },
                events: {
                  'input': (event) {
                    final target = event.target! as web.HTMLInputElement;
                    setState(() => _newTag = target.value);
                  },
                  'keydown': (event) {
                    if ((event as web.KeyboardEvent).key == 'Enter') {
                      event.preventDefault();
                      _addTag();
                    }
                  },
                },
              ),
            ]),
            div(classes: 'control', [
              button(
                classes: 'button is-small is-link',
                disabled: _isBusy,
                attributes: const {'title': 'Add this tag'},
                onClick: _addTag,
                const [
                  span(classes: 'icon', [i(classes: 'fas fa-plus', [])]),
                ],
              ),
            ]),
          ]),
      ],
      actions: [
        a(
          href: file.url,
          classes: 'button is-primary is-outlined',
          attributes: const {'download': ''},
          const [IconLabel(icon: 'download', label: 'Download')],
        ),
        if (component.api.canDeleteFiles)
          BulmaButton(
            color: BulmaColor.danger,
            onPressed: _isBusy ? null : _delete,
            child: const IconLabel(icon: 'trash-alt', label: 'Delete'),
          ),
        BulmaButton(
          onPressed: component.onClose,
          child: const Component.text('Close'),
        ),
      ],
    );
  }

  /// One `label -> value` line of the properties table.
  Component _row(String label, Component value) {
    return div(classes: 'is-flex is-justify-content-space-between mb-2', [
      span(classes: 'has-text-grey is-size-7 mr-4', [Component.text(label)]),
      span(classes: 'has-text-right is-size-7', [value]),
    ]);
  }

  Component _copyButton({
    required String title,
    required VoidCallback onPressed,
  }) {
    return button(
      classes: 'button is-small is-white ml-2',
      attributes: {'title': title},
      onClick: onPressed,
      const [
        span(classes: 'icon is-small', [i(classes: 'fas fa-copy', [])]),
      ],
    );
  }

  /// A tag chip: clicking the tag copies its `/t/{tag}` link, the × removes it.
  Component _tagChip(String tag) {
    final tagLink = _absoluteUrl('/t/$tag');

    return span(classes: 'tag is-link is-light', [
      span(
        classes: 'has-text-weight-medium',
        attributes: {'style': 'cursor: pointer;', 'title': 'Copy $tagLink'},
        events: {
          'click': (event) {
            event.stopPropagation();
            _copy(tagLink, 'Tag link');
          },
        },
        [Component.text(tag)],
      ),
      if (component.api.canRemoveTags)
        button(
          classes: 'delete is-small ml-2',
          attributes: const {'title': 'Remove this tag', 'type': 'button'},
          onClick: () => _removeTag(tag),
          const [],
        ),
    ]);
  }
}
