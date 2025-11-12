import 'package:jaspr/jaspr.dart';

class Column extends StatelessComponent {
  const Column(this.children, {super.key});

  final List<Component> children;

  @override
  Component build(BuildContext context) {
    return div([for (final child in children) child]);
  }
}
