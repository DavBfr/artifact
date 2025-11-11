import 'package:jaspr/jaspr.dart';
import 'package:universal_web/web.dart' as web;

import '../models/api.dart';
import '../models/api_models.dart';
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
  var _api = ArtifactApiClient.base();
  ConfigResponse? _config;
  List<FileInfo>? _files;
  var _altPressed = false;
  bool _isUploading = false;
  String? _uploadingFileName;
  int _uploadProgress = 0;

  @override
  void initState() {
    super.initState();
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
    } on FileTooLargeException catch (e) {
      setState(() {
        _isUploading = false;
        _uploadingFileName = null;
        _uploadProgress = 0;
      });
      print('File too large: ${e.message}');
      // Show error notification
    } on AuthenticationException catch (e) {
      setState(() {
        _isUploading = false;
        _uploadingFileName = null;
        _uploadProgress = 0;
      });
      print('Authentication error: ${e.message}');
      // Show error notification
    } on ApiException catch (e) {
      setState(() {
        _isUploading = false;
        _uploadingFileName = null;
        _uploadProgress = 0;
      });
      print('Upload failed: ${e.message}');
      // Show error notification
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
          onAuthToggle: (value) {
            print('Auth toggle: $value');
            if (value) {
              _api = ArtifactApiClient.base(
                authToken: 'HjYIeiJXFSqI9azevpU5x0B57wym4MrE',
              );
            } else {
              _api = ArtifactApiClient.base();
              _config = null;
            }
            _load();
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
            onDelete: (file) {
              _api.deleteFile(file);
            },
          ),
        ],

        // Auth Modal
        // AuthModal(onLogin: (String p1) {}),

        // Toast notifications
        // Toast(),
      ])),
    );
  }
}
