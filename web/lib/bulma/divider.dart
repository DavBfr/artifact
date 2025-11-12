import 'package:jaspr/jaspr.dart';

class Divider extends StatelessComponent {
  const Divider({super.key});

  @override
  Component build(BuildContext context) {
    return div(classes: 'is-divider', []);
  }
}
