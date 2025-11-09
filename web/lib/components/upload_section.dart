import 'package:jaspr/jaspr.dart';

import '../utils/formatters.dart';

class UploadSection extends StatefulComponent {
  final bool isVisible;
  final int maxContentLength;
  final bool isUploading;
  final String? uploadingFileName;
  final int uploadProgress;
  final void Function(String) onUpload;
  final String? authToken;

  const UploadSection({
    required this.isVisible,
    required this.maxContentLength,
    required this.isUploading,
    this.uploadingFileName,
    required this.uploadProgress,
    required this.onUpload,
    this.authToken,
    super.key,
  });

  @override
  State<UploadSection> createState() => _UploadSectionState();
}

class _UploadSectionState extends State<UploadSection> {
  @override
  void initState() {
    super.initState();
    _setupUploadHandlers();
  }

  void _setupUploadHandlers() {
    // Set up handlers after the component is mounted
    // Future.delayed(Duration.zero, () {
    // final uploadZone = html.document.getElementById('upload-zone');
    // final fileInput =
    //     html.document.getElementById('file-input')
    //         as html.FileUploadInputElement?;
    // final selectBtn = html.document.querySelector(
    //   '.upload-zone button.btn-primary',
    // );
    // final copyBtn = html.document.querySelector('.upload-zone button.btn-sm');

    // if (uploadZone != null && fileInput != null) {
    //   // Drag and drop handlers
    //   uploadZone.onDragOver.listen((event) {
    //     event.preventDefault();
    //     uploadZone.classes.add('dragover');
    //   });

    //   uploadZone.onDragLeave.listen((event) {
    //     event.preventDefault();
    //     uploadZone.classes.remove('dragover');
    //   });

    //   uploadZone.onDrop.listen((event) {
    //     event.preventDefault();
    //     uploadZone.classes.remove('dragover');
    //     final files = event.dataTransfer.files;
    //     if (files != null) {
    //       for (var file in files) {
    //         component.onUpload(file);
    //       }
    //     }
    //   });

    //   // File input handler
    //   fileInput.onChange.listen((event) {
    //     final files = fileInput.files;
    //     if (files != null) {
    //       for (var file in files) {
    //         component.onUpload(file);
    //       }
    //     }
    //   });
    // }

    // Select files button
    // selectBtn?.onClick.listen((event) {
    //   fileInput?.click();
    // });

    // // Copy curl command button
    // copyBtn?.onClick.listen((event) {
    //   event.stopPropagation();
    //   _copyCurlCommand();
    // });
    // });
  }

  void _copyCurlCommand() {
    // final serverUrl = html.window.location.origin;
    // final token = component.authToken ?? '{token}';
    // final curlCommand =
    //     'curl --progress-bar -H \'Authorization: Bearer $token\' $serverUrl/api/upload -F \'file=@example.zip\' | cat';

    // html.window.navigator.clipboard?.writeText(curlCommand);
    // _showNotification('Curl command copied to clipboard!', 'success');
  }

  void _showNotification(String message, String type) {
    // final toast = html.document.getElementById('notification-toast');
    // final toastBody = toast?.querySelector('.toast-body');
    // final toastIcon = toast?.querySelector('.bi');

    // if (toastBody != null) {
    //   toastBody.text = message;
    // }

    // if (toastIcon != null) {
    //   toastIcon.className = type == 'success'
    //       ? 'bi bi-check-circle text-success me-2'
    //       : type == 'error'
    //       ? 'bi bi-exclamation-circle text-danger me-2'
    //       : 'bi bi-info-circle text-primary me-2';
    // }

    // if (toast != null) {
    //   // Use dynamic approach to call Bootstrap's Toast
    //   final dynamic toastElement = toast;
    //   final dynamic bootstrap = html.window as dynamic;
    //   try {
    //     bootstrap.bootstrap?.Toast(toastElement)?.show();
    //   } catch (e) {
    //     // Fallback - Bootstrap might not be loaded yet
    //     print('Failed to show toast: $e');
    //   }
    // }
  }

  @override
  Component build(BuildContext context) {
    return div(
      classes: 'row mb-4',
      id: 'upload-section',
      attributes: {
        'style': component.isVisible ? 'display: block;' : 'display: none;',
      },
      [
        div(classes: 'col-12', [
          div(classes: 'card', [
            div(classes: 'card-header', [
              h5(classes: 'card-title mb-0', [
                i(classes: 'bi bi-cloud-upload', []),
                text(' Upload Artifact'),
              ]),
            ]),
            div(classes: 'card-body', [
              div(classes: 'upload-zone', id: 'upload-zone', [
                i(classes: 'bi bi-cloud-upload fs-1 text-muted mb-3', []),
                h5([text('Drop files here or click to browse')]),
                p(classes: 'text-muted', id: 'max-size-text', [
                  text(
                    'Maximum file size: ${formatBytes(component.maxContentLength)}',
                  ),
                ]),
                small(classes: 'text-muted d-block mb-2', [
                  text('Upload via curl '),
                  button(
                    classes: 'btn btn-sm',
                    attributes: {'title': 'Copy to clipboard'},
                    [i(classes: 'bi bi-clipboard', [])],
                  ),
                ]),
                input(
                  type: InputType.file,
                  id: 'file-input',
                  classes: 'd-none',
                  attributes: {'multiple': 'true'},
                ),
                button(classes: 'btn btn-primary', [
                  i(classes: 'bi bi-plus-circle', []),
                  text(' Select Files'),
                ]),
              ]),
              // Upload Progress
              div(
                classes: 'upload-progress mt-3',
                attributes: {
                  'style': component.isUploading
                      ? 'display: block;'
                      : 'display: none;',
                },
                [
                  div(classes: 'd-flex justify-content-between mb-2', [
                    span(id: 'upload-filename', [
                      text(component.uploadingFileName ?? 'Uploading...'),
                    ]),
                    span(id: 'upload-percentage', [
                      text('${component.uploadProgress}%'),
                    ]),
                  ]),
                  div(classes: 'progress', [
                    div(
                      classes:
                          'progress-bar progress-bar-striped progress-bar-animated',
                      id: 'upload-progress-bar',
                      attributes: {
                        'role': 'progressbar',
                        'style': 'width: ${component.uploadProgress}%',
                      },
                      [],
                    ),
                  ]),
                ],
              ),
            ]),
          ]),
        ]),
      ],
    );
  }
}
