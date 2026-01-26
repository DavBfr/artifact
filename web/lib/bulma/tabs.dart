import 'package:jaspr/dom.dart';
import 'package:jaspr/jaspr.dart';

/// Bulma Tabs Component
/// Supports a limited subset of the available options
/// See https://bulma.io/documentation/components/tabs/ for a detailed description
class BulmaTabs extends StatefulComponent {
  const BulmaTabs({
    required this.tabs,
    required this.onSelected,
    this.isBoxed = false,
    this.isToggle = false,
    super.key,
  });

  final List<Component> tabs;
  final ValueChanged<int> onSelected;
  final bool isBoxed;
  final bool isToggle;

  @override
  State<BulmaTabs> createState() => _TabsState();
}

class _TabsState extends State<BulmaTabs> {
  var selected = 0;

  @override
  Component build(BuildContext context) {
    return div(
      classes:
          'tabs'
          '${component.isBoxed ? ' is-boxed' : ''}'
          '${component.isToggle ? ' is-toggle' : ''}',
      [
        ul([
          for (var i = 0; i < component.tabs.length; i++)
            BulmaTab(
              selected: i == selected,
              onSelected: () {
                setState(() => selected = i);
                component.onSelected(i);
              },
              child: component.tabs[i],
            ),
        ]),
      ],
    );
  }
}

class BulmaTab extends StatelessComponent {
  const BulmaTab({
    required this.selected,
    required this.onSelected,
    required this.child,
    super.key,
  });

  final bool selected;
  final VoidCallback onSelected;
  final Component child;

  @override
  Component build(BuildContext context) {
    return li(classes: selected ? 'is-active' : null, [
      a(href: '#', onClick: onSelected, [child]),
    ]);
  }
}
