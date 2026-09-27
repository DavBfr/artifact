import 'dart:async';

import 'package:jaspr/dom.dart';
import 'package:jaspr/jaspr.dart';
import 'package:universal_web/web.dart' as web;

import '../bulma/bulma.dart';
import '../models/api.dart';
import '../models/api_models.dart';
import '../utils/session_info.dart';
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

  /// Whether /api/config has answered yet. Until it has, the app cannot know
  /// whether a provider login is on offer, so the login button waits rather
  /// than guessing and showing a token form.
  bool _configLoaded = false;

  /// Who the stored session token says we are. Display only.
  SessionInfo? _session;

  /// The error code from a failed provider login, reported once the app is up.
  String? _pendingAuthError;

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

    // A provider login hands the session token back in the URL fragment, and
    // the fragment is part of the page URL the API client derives its base URL
    // from, so this has to run before the client is built.
    _pendingAuthError = SessionInfo.consumeLoginFragment(context);

    // Initialize API client with stored token if available
    final storedToken = TokenStorage.getToken(context);
    _api = ArtifactApiClient.base(authToken: storedToken);
    _session = SessionInfo.fromToken(storedToken);

    if (kIsWeb) {
      _load();
    }
  }

  /// The configuration to fall back to when there is no valid session. It keeps
  /// what the server said about itself - notably whether OIDC is on - because
  /// that answer does not depend on being signed in.
  ConfigResponse get _signedOutConfig => ConfigResponse(
    success: true,
    oidcEnabled: _config.oidcEnabled,
    filenameUrlsEnabled: _config.filenameUrlsEnabled,
  );

  /// Turns a callback error code into something worth reading.
  String _describeAuthError(String code) {
    switch (code) {
      case 'state_missing':
      case 'state_invalid':
      case 'state_mismatch':
        return 'The login response did not match this browser session. Please try again.';
      case 'state_expired':
        return 'The login took too long to complete. Please try again.';
      case 'invalid_id_token':
      case 'nonce_mismatch':
        return 'The identity provider returned an unexpected token. Please try again.';
      case 'provider_unavailable':
        return 'The identity provider could not be reached. Please try again later.';
      case 'provider_error':
        return 'The identity provider refused the login.';
      default:
        return 'Login failed ($code). Please try again.';
    }
  }

  Future<void> _load() async {
    // A login the provider rejected comes back as a fragment rather than a
    // token, and is worth telling the user about.
    final authError = _pendingAuthError;
    if (authError != null) {
      _pendingAuthError = null;
      NotificationMessenger.of(context).showNotification(
        BulmaNotification.error(
          _describeAuthError(authError),
          title: 'Login Failed',
        ),
      );
    }

    try {
      final configResponse = await _api.getConfig();
      setState(() {
        _config = configResponse;
        _configLoaded = true;
      });
    } on AuthenticationException {
      // A stored credential was rejected - drop it and fall back to the
      // unauthenticated view.
      TokenStorage.removeToken(context);
      setState(() {
        _config = _signedOutConfig;
        _configLoaded = true;
        _api = ArtifactApiClient.base();
        _session = null;
      });

      // Show error notification
      NotificationMessenger.of(context).showNotification(
        BulmaNotification.error(
          _config.oidcEnabled
              ? 'Your session has expired. Please sign in again.'
              : 'Invalid authentication token. Please login again.',
          title: 'Authentication Error',
        ),
      );
    } catch (e) {
      setState(() {
        _config = _signedOutConfig;
        _configLoaded = true;
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
        if (_configLoaded && _files != null)
          NavBar(
            isAuthenticated: _api.isAuthenticated,
            altPressed: _altPressed,
            sessionLabel: _session?.label,
            showTitleAndRefresh: !(_listingRestricted && !_api.isAuthenticated),
            onAuthToggle: (value) async {
              if (value) {
                await _login();
              } else {
                // Logout: remove token and reset API client
                TokenStorage.removeToken(context);
                setState(() {
                  _api = ArtifactApiClient.base();
                  _session = null;
                  _config = _signedOutConfig;
                });
                await _load();
              }
            },
            onRefresh: _load,
          ),

        if (!_configLoaded || (_listingRestricted && !_api.isAuthenticated))
          _buildRestrictedView()
        else if (_files == null)
          const MyLoading()
        else ...[
          // Stats
          StatsCard(stats: _stats),

          if (_api.canCreateFiles)
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
            api: _api,
            onDelete: _delete,
            onRefresh: _load,
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
    } on PermissionException catch (e) {
      setState(() {
        _isUploading = false;
        _uploadingFileName = null;
        _uploadProgress = 0;
      });

      // Replacements and tags need more than file:create, so the server's
      // message says which permission this upload was missing.
      NotificationMessenger.of(context).showNotification(
        BulmaNotification.error(e.message, title: 'Not Permitted'),
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
    // Whether a token form is even the right thing to offer comes from the
    // server, so make sure that answer has arrived before deciding.
    if (!_configLoaded) {
      await _load();
    }

    // With OIDC the browser goes to the provider and comes back to the callback;
    // there is no token for the user to type, so no dialog is offered. The token
    // form remains reachable only when no provider is configured.
    if (_config.oidcEnabled) {
      web.window.location.assign('${_api.baseUrl}/api/auth/login');
      return;
    }

    // Show login dialog using DialogManager
    final token = await DialogManager.of(context).showDialog<String>(
      (onComplete) => AuthDialog(onLogin: onComplete, onCancel: onComplete),
    );

    if (token == null || token.isEmpty) return;

    TokenStorage.saveToken(context, token);
    setState(() {
      _api = ArtifactApiClient.base(authToken: token);
      _session = SessionInfo.fromToken(token);
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
