import 'package:artifact_web/components/bulma_dialogs.dart';
import 'package:artifact_web/components/bulma_notifications.dart';
import 'package:jaspr/jaspr.dart';

import 'components/app.dart';

void main() {
  runApp(
    NotificationMessengerProvider(child: DialogManagerProvider(child: App())),
  );
}
