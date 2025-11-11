import 'package:jaspr/jaspr.dart';

class BulmaLevel extends StatelessComponent {
  const BulmaLevel(this.children, {super.key});

  final List<Component> children;

  @override
  Component build(BuildContext context) {
    return nav(classes: 'level', children);
  }
}

/// Level Item
class BulmaLevelItem extends StatelessComponent {
  const BulmaLevelItem({
    this.title,
    this.heading,
    this.isCentered = true,
    super.key,
  });

  final Component? title;
  final Component? heading;
  final bool isCentered;

  @override
  Component build(BuildContext context) {
    return div(classes: 'level-item${isCentered ? ' has-text-centered' : ''}', [
      div([
        if (heading != null) p(classes: 'heading', [heading!]),
        if (title != null) p(classes: 'title', [title!]),
      ]),
    ]);
  }
}
