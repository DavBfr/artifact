import 'package:jaspr/dom.dart';
import 'package:jaspr/jaspr.dart';
import 'package:universal_web/js_interop.dart';
import 'package:universal_web/web.dart';

import '../bulma/bulma.dart';
import '../utils/formatters.dart';

class UploadSection extends StatefulComponent {
  const UploadSection({
    required this.maxContentLength,
    required this.isUploading,
    this.uploadingFileName,
    required this.uploadProgress,
    required this.onUpload,
    this.authToken,
    super.key,
  });

  final int maxContentLength;
  final bool isUploading;
  final String? uploadingFileName;
  final int uploadProgress;
  final void Function(File) onUpload;
  final String? authToken;

  @override
  State<UploadSection> createState() => _UploadSectionState();
}

class _UploadSectionState extends State<UploadSection> {
  bool isDragOver = false;

  @override
  void initState() {
    super.initState();
    Future.delayed(Duration.zero, _setupUploadHandlers);
  }

  void _setupUploadHandlers() {
    final uploadZone = document.getElementById('upload-zone');
    final fileInput =
        document.getElementById('file-input') as HTMLInputElement?;

    if (uploadZone != null && fileInput != null) {
      // Drag and drop handlers
      uploadZone.addEventListener('dragover', _onDragOver.toJS);
      uploadZone.addEventListener('dragleave', _onDragLeave.toJS);
      uploadZone.addEventListener('drop', _onDrop.toJS);

      // File input change handler
      fileInput.addEventListener('change', _onFileInputChange.toJS);

      // Click zone to trigger file input
      uploadZone.addEventListener(
        'click',
        ((Event event) {
          fileInput.click();
        }).toJS,
      );
    }

    // Copy curl command button
    final copyBtn = document.getElementById('copy-curl-btn');
    copyBtn?.addEventListener(
      'click',
      ((Event event) {
        event.stopPropagation();
        _copyCurlCommand();
      }).toJS,
    );
  }

  void _onDragOver(Event event) {
    event.preventDefault();
    setState(() {
      isDragOver = true;
    });
  }

  void _onDragLeave(Event event) {
    event.preventDefault();
    setState(() {
      isDragOver = false;
    });
  }

  void _onDrop(Event event) {
    event.preventDefault();
    setState(() {
      isDragOver = false;
    });

    final dragEvent = event as DragEvent;
    final files = dragEvent.dataTransfer?.files;

    if (files != null && files.length > 0) {
      for (var i = 0; i < files.length; i++) {
        final file = files.item(i);
        if (file != null) {
          component.onUpload(file);
        }
      }
    }
  }

  void _onFileInputChange(Event event) {
    final input = event.target as HTMLInputElement?;
    final files = input?.files;

    if (files != null && files.length > 0) {
      for (var i = 0; i < files.length; i++) {
        final file = files.item(i);
        if (file != null) {
          component.onUpload(file);
        }
      }
    }
  }

  void _copyCurlCommand() {
    final serverUrl = window.location.origin;
    final token = component.authToken ?? '{token}';
    final curlCommand =
        'curl --progress-bar -H "Authorization: Bearer $token" $serverUrl/api/upload -F "file=@example.zip" | cat';

    window.navigator.clipboard.writeText(curlCommand);
    _showNotification('Curl command copied to clipboard!');
  }

  void _showNotification(String message) {
    NotificationMessenger.of(context).showNotification(
      BulmaNotification(
        title: const Component.text('Copied!'),
        child: Component.text(message),
        type: NotificationType.info,
      ),
    );
  }

  @override
  void dispose() {
    final uploadZone = document.getElementById('upload-zone');
    final fileInput =
        document.getElementById('file-input') as HTMLInputElement?;

    if (uploadZone != null) {
      uploadZone.removeEventListener('dragover', _onDragOver.toJS);
      uploadZone.removeEventListener('dragleave', _onDragLeave.toJS);
      uploadZone.removeEventListener('drop', _onDrop.toJS);
    }

    if (fileInput != null) {
      fileInput.removeEventListener('change', _onFileInputChange.toJS);
    }

    super.dispose();
  }

  @override
  Component build(BuildContext context) {
    final dragOverClass = isDragOver ? ' has-background-primary-light' : '';

    return div(classes: 'box has-background-white mb-5', id: 'upload-section', [
      // Header
      nav(classes: 'level mb-4', [
        const div(classes: 'level-left', [
          div(classes: 'level-item', [
            div([
              p(classes: 'title is-4 mb-1', [
                span(classes: 'icon-text', [
                  span(classes: 'icon has-text-info', [
                    i(classes: 'fas fa-cloud-upload-alt', []),
                  ]),
                  span([Component.text('Upload Artifact')]),
                ]),
              ]),
            ]),
          ]),
        ]),
        div(classes: 'level-right', [
          div(classes: 'level-item', [
            span(classes: 'tag is-info is-light is-medium', [
              const span(classes: 'icon', [
                i(classes: 'fas fa-info-circle', []),
              ]),
              span([
                Component.text(
                  'Max: ${formatBytes(component.maxContentLength)}',
                ),
              ]),
            ]),
          ]),
        ]),
      ]),

      // Upload zone
      div(
        classes:
            'box has-background-light has-text-centered p-6 is-clickable$dragOverClass',
        id: 'upload-zone',
        attributes: const {
          'style':
              'border: 2px dashed #dbdbdb; border-radius: 6px; transition: all 0.3s ease;',
        },
        [
          const div(classes: 'mb-4', [
            span(classes: 'icon is-large has-text-info', [
              i(classes: 'fas fa-cloud-upload-alt fa-3x', []),
            ]),
          ]),
          const p(classes: 'title is-5 has-text-grey-dark mb-4', [
            Component.text('Drop files here or click to browse'),
          ]),
          const p(classes: 'subtitle is-6 has-text-grey-light mb-6', [
            Component.text('Supports multiple file uploads'),
          ]),

          // Hidden file input
          const input(
            type: InputType.file,
            id: 'file-input',
            attributes: {'multiple': 'true', 'style': 'display: none;'},
          ),

          // Curl command section
          div(classes: 'box is-shadowless has-background-white-ter mt-4 p-3', [
            const p(classes: 'is-size-7 has-text-grey mb-2', [
              span(classes: 'icon-text', [
                span(classes: 'icon is-small', [
                  i(classes: 'fas fa-terminal', []),
                ]),
                span([Component.text('Upload via curl command')]),
              ]),
            ]),
            div(classes: 'field has-addons', [
              div(classes: 'control is-expanded', [
                input(
                  type: InputType.text,
                  classes: 'input is-small is-family-monospace',
                  events: {'click': (Event e) => e.stopPropagation()},
                  attributes: const {
                    'value':
                        'curl -H "Authorization: Bearer {token}" ... -F "file=@example.zip"',
                    'readonly': 'true',
                  },
                ),
              ]),
              const div(classes: 'control', [
                button(
                  classes: 'button is-info is-small',
                  id: 'copy-curl-btn',
                  attributes: {'title': 'Copy to clipboard'},
                  [
                    span(classes: 'icon is-small', [
                      i(classes: 'fas fa-clipboard', []),
                    ]),
                  ],
                ),
              ]),
            ]),
          ]),
        ],
      ),

      // Upload Progress
      if (component.isUploading)
        div(classes: 'box is-shadowless has-background-light mt-4', [
          div(classes: 'mb-3', [
            nav(classes: 'level is-mobile', [
              div(classes: 'level-left', [
                div(classes: 'level-item', [
                  p(classes: 'subtitle is-6 has-text-weight-semibold', [
                    span(classes: 'icon-text', [
                      const span(classes: 'icon has-text-info', [
                        i(classes: 'fas fa-spinner fa-pulse', []),
                      ]),
                      span(id: 'upload-filename', [
                        Component.text(
                          component.uploadingFileName ?? 'Uploading...',
                        ),
                      ]),
                    ]),
                  ]),
                ]),
              ]),
              div(classes: 'level-right', [
                div(classes: 'level-item', [
                  span(classes: 'tag is-info', id: 'upload-percentage', [
                    Component.text('${component.uploadProgress}%'),
                  ]),
                ]),
              ]),
            ]),
          ]),
          progress(
            classes: 'progress is-info',
            id: 'upload-progress-bar',
            attributes: {'value': '${component.uploadProgress}', 'max': '100'},
            [Component.text('${component.uploadProgress}%')],
          ),
        ]),
    ]);
  }
}
