import 'dart:async';

import 'package:jaspr/dom.dart';
import 'package:jaspr/jaspr.dart';
import 'package:universal_web/web.dart' as web;

import '../bulma/bulma.dart';
import '../models/api.dart';
import '../models/api_models.dart';
import '../utils/token_storage.dart';
import 'auth_dialog.dart';
import 'files_list.dart';
import 'key_listener.dart';
import 'loading.dart';
import 'logo.dart';
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
  ConfigResponse _config = ConfigResponse.empty;
  StatsResponse? _stats;
  List<FileInfo>? _files;
  bool _hasMore = true;
  String _searchQuery = '';
  bool _isLoadingMore = false;
  Timer? _searchDebounce;
  bool _listingRestricted = false;
  var _altPressed = false;
  bool _isUploading = false;
  String? _uploadingFileName;
  int _uploadProgress = 0;

  @override
  void initState() {
    super.initState();

    // Initialize API client with stored token if available
    final storedToken = TokenStorage.getToken(context);
    _api = ArtifactApiClient.base(authToken: storedToken);

    if (kIsWeb) {
      _load();
    }
  }

  Future<void> _load() async {
    try {
      final configResponse = await _api.getConfig();
      setState(() {
        _config = configResponse;
      });
    } on AuthenticationException {
      // A stored token was rejected - show notification and logout.
      TokenStorage.removeToken(context);
      setState(() {
        _config = ConfigResponse.empty;
        _api = ArtifactApiClient.base();
      });

      // Show error notification
      NotificationMessenger.of(context).showNotification(
        BulmaNotification.error(
          'Invalid authentication token. Please login again.',
          title: 'Authentication Error',
        ),
      );
    } catch (e) {
      setState(() {
        _config = ConfigResponse.empty;
      });
      NotificationMessenger.of(context).showNotification(
        BulmaNotification.error(
          'Failed to load configuration. Please try again. $e',
          title: 'Error',
        ),
      );
    }

    try {
      final filesResponse = await _api.listFiles(
        limit: _config.pageSize,
        search: _searchQuery,
      );
      setState(() {
        _files = filesResponse.files;
        _hasMore = filesResponse.count == _config.pageSize;
        _listingRestricted = false;
      });
    } on AuthenticationException {
      // Listing disabled for unauthenticated users (ART_NO_LISTING) - show an
      // empty list instead of leaving the UI stuck loading forever.
      setState(() {
        _files = [];
        _hasMore = false;
        _listingRestricted = true;
      });
    } catch (e) {
      setState(() {
        _files = [];
        _hasMore = false;
      });
      NotificationMessenger.of(context).showNotification(
        BulmaNotification.error(
          'Failed to load file list. Please try again.',
          title: 'Error',
        ),
      );
    }

    try {
      final statsResponse = await _api.getStats();
      setState(() => _stats = statsResponse);
    } catch (e) {
      // Stats are supplementary - leave the previous value rather than erroring the page.
    }
  }

  /// Fetches the next page of results (server-side pagination) and appends it,
  /// stopping once a page comes back short of a full page (count < limit).
  Future<void> _loadMore() async {
    final files = _files;
    if (_isLoadingMore || files == null || !_hasMore) return;

    setState(() => _isLoadingMore = true);
    try {
      final filesResponse = await _api.listFiles(
        limit: _config.pageSize,
        offset: files.length,
        search: _searchQuery,
      );
      setState(() {
        _files = [...files, ...filesResponse.files];
        _hasMore = filesResponse.count == _config.pageSize;
        _isLoadingMore = false;
      });
    } catch (e) {
      setState(() => _isLoadingMore = false);
      NotificationMessenger.of(context).showNotification(
        BulmaNotification.error(
          'Failed to load more files. Please try again.',
          title: 'Error',
        ),
      );
    }
  }

  /// Debounces the search box so we don't hit the API on every keystroke.
  void _onSearchChanged(String value) {
    _searchDebounce?.cancel();
    _searchDebounce = Timer(const Duration(milliseconds: 300), () {
      setState(() => _searchQuery = value);
      _load();
    });
  }

  @override
  void dispose() {
    _searchDebounce?.cancel();
    super.dispose();
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
      div(classes: 'container', [
        NavBar(
          isAuthenticated: _api.isAuthenticated,
          altPressed: _altPressed,
          showTitleAndRefresh: !(_listingRestricted && !_api.isAuthenticated),
          onAuthToggle: (value) async {
            if (value) {
              await _login();
            } else {
              // Logout: remove token and reset API client
              TokenStorage.removeToken(context);
              setState(() {
                _api = ArtifactApiClient.base();
                _config = ConfigResponse.empty;
              });
              await _load();
            }
          },
          onRefresh: _load,
        ),

        if (_files == null)
          const MyLoading()
        else if (_listingRestricted && !_api.isAuthenticated)
          _buildRestrictedView()
        else ...[
          // Stats
          StatsCard(stats: _stats),

          if (_api.isAuthenticated)
            UploadSection(
              maxContentLength: _config.maxContentLength,
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
            onDelete: _delete,
            searchQuery: _searchQuery,
            onSearchChanged: _onSearchChanged,
            hasMore: _hasMore,
            isLoadingMore: _isLoadingMore,
            onLoadMore: _loadMore,
            filenameUrlsEnabled: _config.filenameUrlsEnabled,
          ),
        ],

        const div(classes: 'my-6', []),

        // Footer
        // const BulmaFooter(),
      ]),
    );
  }

  /// Minimal view shown when listing is restricted (ART_NO_LISTING) and the
  /// user isn't authenticated: just the logo and server name, nothing else.
  Component _buildRestrictedView() {
    return div(
      styles: Styles(
        display: Display.flex,
        flexDirection: FlexDirection.column,
        alignItems: AlignItems.center,
        justifyContent: JustifyContent.center,
        minHeight: 70.vh,
      ),
      const [
        Logo(size: 160),
        div(classes: 'title mt-4', [Component.text('Artifact Server')]),
      ],
    );
  }

  Future<void> _handleUpload(web.File file) async {
    setState(() {
      _isUploading = true;
      _uploadingFileName = file.name;
      _uploadProgress = 0;
    });

    try {
      await _api.uploadFile(
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

  Future<void> _login() async {
    // Show login dialog using DialogManager
    final token = await DialogManager.of(context).showDialog<String>(
      (onComplete) => AuthDialog(onLogin: onComplete, onCancel: onComplete),
    );

    if (token == null || token.isEmpty) return;

    TokenStorage.saveToken(context, token);
    setState(() {
      _api = ArtifactApiClient.base(authToken: token);
    });
    await _load();
  }

  Future<void> _delete(FileInfo file) async {
    // Show confirmation dialog before deleting
    final result = await DialogManager.of(context).showDialog<bool>(
      (onComplete) => AlertDialog(
        title: const Component.text('Delete File'),
        content: [
          Component.text('Are you sure you want to delete "${file.name}"?'),
          const br(),
          const Component.text('This action cannot be undone.'),
        ],
        actions: [
          BulmaButton(
            child: const Component.text('Delete'),
            color: BulmaColor.danger,
            onPressed: () {
              onComplete(true);
            },
          ),
          BulmaButton(
            child: const Component.text('Cancel'),
            onPressed: () {
              onComplete();
            },
          ),
        ],
      ),
    );

    if (result != true) return;

    try {
      await _api.deleteFile(file.slug);
      await _load();
      NotificationMessenger.of(context).showNotification(
        BulmaNotification.success(
          'File "${file.name}" was deleted successfully.',
          title: 'File Deleted',
        ),
      );
    } catch (e) {
      NotificationMessenger.of(context).showNotification(
        BulmaNotification.error(
          'Failed to delete file "${file.name}". Please try again.',
          title: 'Delete Failed',
        ),
      );
    }
  }
}
