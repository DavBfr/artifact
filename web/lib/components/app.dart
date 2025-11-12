import 'package:artifact_web/components/bulma_button.dart';
import 'package:jaspr/jaspr.dart';
import 'package:universal_web/web.dart' as web;

import '../models/api.dart';
import '../models/api_models.dart';
import '../utils/token_storage.dart';
import 'bulma_dialogs.dart';
import 'bulma_notifications.dart';
import 'files_list.dart';
import 'key_listener.dart';
import 'loading.dart';
import 'navbar.dart';
import 'stats_card.dart';
import 'upload_section.dart';

@client
class App extends StatefulComponent {
  const App({super.key});

  @override
  State<App> createState() => AppState();
}

class AppState extends State<App> {
  late ArtifactApiClient _api;
  ConfigResponse? _config;
  List<FileInfo>? _files;
  var _altPressed = false;
  bool _isUploading = false;
  String? _uploadingFileName;
  int _uploadProgress = 0;

  @override
  void initState() {
    super.initState();

    // Initialize API client with stored token if available
    final storedToken = TokenStorage.getToken();
    _api = ArtifactApiClient.base(authToken: storedToken);

    if (kIsWeb) {
      _load();
    }
  }

  Future<void> _load() async {
    if (_api.isAuthenticated) {
      try {
        final configResponse = await _api.getConfig();
        setState(() {
          _config = configResponse;
        });
      } on AuthenticationException {
        // Invalid token - show notification and logout
        TokenStorage.removeToken();
        setState(() {
          _config = null;
          _api = ArtifactApiClient.base();
        });

        // Show error notification
        NotificationMessenger.of(context).showNotification(
          BulmaNotification.error(
            'Invalid authentication token. Please login again.',
            title: 'Authentication Error',
          ),
        );
      } on ApiException {
        setState(() {
          _config = null;
          _api = ArtifactApiClient.base();
        });
      }
    }

    final filesResponse = await _api.listFiles();
    setState(() {
      _files = filesResponse.files;
    });
  }

  Future<void> _handleUpload(web.File file) async {
    setState(() {
      _isUploading = true;
      _uploadingFileName = file.name;
      _uploadProgress = 0;
    });

    try {
      await _api.uploadFileWithProgressXHR(
        file: file,
        onProgress: (sent, total) {
          final progress = ((sent / total) * 100).round();
          setState(() {
            _uploadProgress = progress;
          });
        },
      );

      // Upload successful, reload files
      await _load();

      setState(() {
        _isUploading = false;
        _uploadingFileName = null;
        _uploadProgress = 0;
      });

      // Show success notification
      NotificationMessenger.of(context).showNotification(
        BulmaNotification.success('File "${file.name}" uploaded successfully!'),
      );
    } on FileTooLargeException catch (e) {
      setState(() {
        _isUploading = false;
        _uploadingFileName = null;
        _uploadProgress = 0;
      });

      // Show error notification
      NotificationMessenger.of(context).showNotification(
        BulmaNotification.error(e.message, title: 'File Too Large'),
      );
    } on AuthenticationException catch (e) {
      setState(() {
        _isUploading = false;
        _uploadingFileName = null;
        _uploadProgress = 0;
      });

      // Show error notification
      NotificationMessenger.of(context).showNotification(
        BulmaNotification.error(e.message, title: 'Authentication Error'),
      );
    } on ApiException catch (e) {
      setState(() {
        _isUploading = false;
        _uploadingFileName = null;
        _uploadProgress = 0;
      });

      // Show error notification
      NotificationMessenger.of(context).showNotification(
        BulmaNotification.error(e.message, title: 'Upload Failed'),
      );
    }
  }

  @override
  Component build(BuildContext context) {
    return KeyListener(
      onKeyDown: (e) {
        if (e.key == 'Alt') {
          setState(() {
            _altPressed = true;
          });
        }
      },
      onKeyUp: (e) {
        if (e.key == 'Alt') {
          setState(() {
            _altPressed = false;
          });
        }
      },
      div(classes: 'container', ([
        NavBar(
          isAuthenticated: _api.isAuthenticated,
          altPressed: _altPressed,
          onAuthToggle: (value) async {
            if (value) {
              // Show login dialog using DialogManager
              final token = await DialogManager.of(context).showDialog<String>(
                (onComplete) => AlertDialog(
                  title: text('API Authentication'),
                  content: [
                    text('Please enter your API token to authenticate.'),
                    input(
                      type: InputType.password,
                      classes: 'input',
                      id: 'token-input',
                      attributes: {
                        'placeholder': 'Enter your API token',
                        'autocomplete': 'off',
                      },
                    ),
                  ],
                  actions: [
                    BulmaButton(
                      child: text('Login'),
                      onPressed: () {
                        final input =
                            web.document.getElementById('token-input')
                                as web.HTMLInputElement?;
                        if (input != null && input.value.trim().isNotEmpty) {
                          onComplete(input.value.trim());
                        }
                      },
                    ),
                    BulmaButton(
                      child: text('Cancel'),
                      onPressed: () {
                        onComplete();
                      },
                    ),
                  ],
                ),
              );

              if (token == null || token.isEmpty) return;

              print('Save Token: $token');
              TokenStorage.saveToken(token);
              setState(() {
                _api = ArtifactApiClient.base(authToken: token);
              });
              await _load();
              NotificationMessenger.of(context).showNotification(
                BulmaNotification.success(
                  'Successfully authenticated! Token stored in browser.',
                  title: 'Login Successful',
                ),
              );
            } else {
              // Logout: remove token and reset API client
              TokenStorage.removeToken();
              setState(() {
                _api = ArtifactApiClient.base();
                _config = null;
              });
              _load();
            }
          },
          onRefresh: () {
            print('Refresh');
            _load();
          },
        ),

        if (_files == null)
          MyLoading()
        else ...[
          // Stats
          StatsCard(files: _files!),

          if (_api.isAuthenticated && _config != null)
            UploadSection(
              maxContentLength: _config!.maxContentLength,
              isUploading: _isUploading,
              uploadingFileName: _uploadingFileName,
              uploadProgress: _uploadProgress,
              onUpload: _handleUpload,
              authToken: _api.authToken,
            ),

          // Files List
          FilesList(
            files: _files!,
            isAuthenticated: _api.isAuthenticated,
            onDelete: (file) async {
              // Show confirmation dialog before deleting
              final result = await DialogManager.of(context).showDialog<bool>(
                (onComplete) => AlertDialog(
                  title: text('Delete File'),
                  content: [
                    text('Are you sure you want to delete "$file"?'),
                    br(),
                    text('This action cannot be undone.'),
                  ],
                  actions: [
                    BulmaButton(
                      child: text('Delete'),
                      onPressed: () {
                        onComplete(true);
                      },
                    ),
                    BulmaButton(
                      child: text('Cancel'),
                      onPressed: () {
                        onComplete();
                      },
                    ),
                  ],
                ),
              );

              if (result != true) return;

              try {
                await _api.deleteFile(file);
                await _load();
                NotificationMessenger.of(context).showNotification(
                  BulmaNotification.success(
                    'File "$file" was deleted successfully.',
                    title: 'File Deleted',
                  ),
                );
              } catch (e) {
                NotificationMessenger.of(context).showNotification(
                  BulmaNotification.error(
                    'Failed to delete file "$file". Please try again.',
                    title: 'Delete Failed',
                  ),
                );
              }
            },
          ),
        ],
      ])),
    );
  }
}
