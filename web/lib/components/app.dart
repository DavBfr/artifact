import 'package:jaspr/jaspr.dart';

import '../models/api.dart';
import '../models/api_models.dart';
import 'auth_modal.dart';
import 'files_list.dart';
import 'loading.dart';
import 'navbar.dart';
import 'stats_card.dart';
import 'toast.dart';
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

  @override
  Component build(BuildContext context) {
    if (_files == null) {
      return MyLoading();
    }

    return div([
      // head([
      //   meta(charset: 'utf-8'),
      //   meta(
      //     name: 'viewport',
      //     content: 'width=device-width, initial-scale=1.0',
      //   ),
      //   raw('<title>Artifact Server</title>'),
      //   // Bootstrap CSS
      //   link(
      //     rel: 'stylesheet',
      //     href:
      //         'https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css',
      //   ),
      //   // Bootstrap Icons
      //   link(
      //     rel: 'stylesheet',
      //     href:
      //         'https://cdn.jsdelivr.net/npm/bootstrap-icons@1.10.0/font/bootstrap-icons.css',
      //   ),
      //   // Custom styles
      //   raw('<style>${AppStyles.customStyles}</style>'),
      // ]),
      // body([
      // Navigation
      NavBar(
        isAuthenticated: _api.isAuthenticated,
        onAuthToggle: () {},
        onRefresh: () {
          print('Refresh');
          _load();
        },
      ),

      // Main container
      div(classes: 'container mt-4', [
        // Stats
        div(classes: 'row mb-4', [
          div(classes: 'col-md-12', [StatsCard(files: _files!)]),
        ]),

        // Upload Section
        UploadSection(
          isVisible: _api.isAuthenticated,
          maxContentLength: 3000,
          isUploading: false,
          uploadProgress: 0,
          onUpload: (file) {},
        ),

        // Files List
        FilesList(
          files: _files!,
          isAuthenticated: _api.isAuthenticated,
          onDelete: (file) {
            _api.deleteFile(file);
          },
        ),
      ]),

      // Auth Modal
      AuthModal(onLogin: (String p1) {}),

      // Toast notifications
      Toast(),

      // Bootstrap JS
      script(
        src:
            'https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/js/bootstrap.bundle.min.js',
      ),
      // ]),
    ]);
  }
}
