import 'package:jaspr/dom.dart';
import 'package:jaspr/jaspr.dart';

class Divider extends StatelessComponent {
  const Divider({super.key});

  @override
  Component build(BuildContext context) {
    return const div(classes: 'is-divider', []);
  }
}
