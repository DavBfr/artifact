import 'package:jaspr/dom.dart';
import 'package:jaspr/jaspr.dart';

import 'colors.dart';

/// Bulma Button Component
/// Supports a limited subset of the available options
/// See https://bulma.io/documentation/elements/button/ for a detailed description
class BulmaButton extends StatelessComponent {
  const BulmaButton({
    required this.child,
    required this.onPressed,
    this.color,
    this.isOutlined = true,
    this.isBlock = false,
    super.key,
  });

  final Component child;
  final VoidCallback? onPressed;
  final BulmaColor? color;
  final bool isBlock;
  final bool isOutlined;

  @override
  Component build(BuildContext context) {
    final isDisabled = onPressed == null;

    return button(
      classes:
          'button'
          '${color != null ? ' is-${color!.name}' : ''}'
          '${isOutlined ? ' is-outlined' : ''}'
          '${isBlock ? ' block' : ''}',
      disabled: isDisabled,
      onClick: onPressed,
      [child],
    );
  }
}

class IconLabel extends StatelessComponent {
  const IconLabel({required this.icon, required this.label, super.key});

  final String icon;
  final String label;

  @override
  Component build(BuildContext context) {
    return Component.fragment([
      span(classes: 'icon', [i(classes: 'fas fa-$icon', const [])]),
      span([Component.text(label)]),
    ]);
  }
}

/// Bulma Button Group Component
class BulmaButtonGroup extends StatelessComponent {
  const BulmaButtonGroup({
    required this.children,
    this.isAttached = false,
    super.key,
  });

  final List<BulmaButton> children;
  final bool isAttached;

  @override
  Component build(BuildContext context) {
    return div(
      classes: 'buttons ${isAttached ? ' has-addons' : ''} block',
      children,
    );
  }
}
