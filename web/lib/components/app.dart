import 'package:jaspr/jaspr.dart';

import '../models/app_state.dart';
import 'auth_modal.dart';
import 'files_list.dart';
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
  @override
  void initState() {
    super.initState();
    // Run code depending on the rendering environment.
    if (kIsWeb) {
      print("Hello client");
      // When using @client components there is no default `main()` function on the client where you would normally
      // run any client-side initialization logic. Instead you can put it here, considering this component is only
      // mounted once at the root of your client-side component tree.
    } else {
      print("Hello server");
    }
  }

  @override
  Component build(BuildContext context) {
    final files = <FileInfo>[
      FileInfo(
        name: 'example1.zip',
        size: 1500000,
        modified: DateTime.now()
            .subtract(Duration(minutes: 10))
            .toIso8601String(),
        url: 'https://example.com/example1.zip',
      ),
      FileInfo(
        name: 'example2.tar.gz',
        size: 25000000,
        modified: DateTime.now().subtract(Duration(hours: 2)).toIso8601String(),
        url: 'https://example.com/example2.tar.gz',
      ),
    ];

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
        isAuthenticated: true,
        onAuthToggle: () {},
        onRefresh: () {
          print('Refresh');
          ;
        },
      ),

      // Main container
      div(classes: 'container mt-4', [
        // Stats
        div(classes: 'row mb-4', [
          div(classes: 'col-md-12', [StatsCard(files: files)]),
        ]),

        // Upload Section
        UploadSection(
          isVisible: true,
          maxContentLength: 3000,
          isUploading: false,
          uploadProgress: 0,
          onUpload: (p1) {},
        ),

        // Files List
        FilesList(
          files: files,
          isLoading: false,
          isAuthenticated: false,
          onDelete: (file) {},
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
