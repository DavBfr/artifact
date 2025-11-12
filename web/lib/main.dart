import 'package:jaspr/jaspr.dart';

import 'bulma/bulma.dart';
import 'widgets/app.dart';

void main() {
  runApp(
    const NotificationMessengerProvider(
      child: DialogManagerProvider(child: App()),
    ),
  );
}
