import 'package:jaspr/jaspr.dart';

class Row extends StatelessComponent {
  const Row(this.children, {super.key});

  final List<Component> children;

  @override
  Component build(BuildContext context) {
    return div(classes: 'columns is-vcentered', [
      for (final child in children) div(classes: 'column', [child]),
    ]);
  }
}
