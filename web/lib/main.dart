import 'package:jaspr/jaspr.dart';

import 'components/app.dart';
import 'components/bulma_dialogs.dart';
import 'components/bulma_notifications.dart';

void main() {
  runApp(
    const NotificationMessengerProvider(child: DialogManagerProvider(child: App())),
  );
}
