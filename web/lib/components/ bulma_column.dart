import 'package:jaspr/jaspr.dart';

class BulmaColumn extends StatelessComponent {
  const BulmaColumn(this.children, {super.key});

  final List<Component> children;

  @override
  Component build(BuildContext context) {
    return div(classes: 'columns', [
      for (final child in children) div(classes: 'column', [child]),
    ]);
  }
}
