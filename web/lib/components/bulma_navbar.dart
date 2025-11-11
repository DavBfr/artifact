import 'package:jaspr/jaspr.dart';

/// Bulma Navbar Component
/// Supports a limited subset of the available options
/// See https://bulma.io/documentation/components/navbar/ for a detailed description
class BulmaNavBar extends StatelessComponent {
  const BulmaNavBar(this.children, {super.key});

  final List<Component> children;

  @override
  Component build(BuildContext context) {
    return nav(classes: 'navbar block', children);
  }
}

class BulmaNavbarBrand extends StatelessComponent {
  const BulmaNavbarBrand({required this.children, super.key});

  final List<Component> children;

  @override
  Component build(BuildContext context) {
    return div(classes: 'navbar-brand', children);
  }
}

class BulmaNavbarBurger extends StatelessComponent {
  const BulmaNavbarBurger({required this.isActive, required this.onToggle});

  final bool isActive;
  final void Function() onToggle;

  @override
  Component build(BuildContext context) {
    return button(
      classes: "navbar-burger${isActive ? ' is-active' : ''}",
      attributes: {"role": "button", "data-target": "navMenu"},
      events: events(
        onClick: () {
          onToggle();
        },
      ),
      [
        span(attributes: {"aria-hidden": "true"}, []),
        span(attributes: {"aria-hidden": "true"}, []),
        span(attributes: {"aria-hidden": "true"}, []),
      ],
    );
  }
}

class BulmaNavbarMenu extends StatelessComponent {
  const BulmaNavbarMenu({
    this.isActive = false,
    required this.items,
    this.endItems = const [],
    super.key,
  });

  final bool isActive;
  final List<Component> items;
  final List<Component> endItems;

  @override
  Component build(BuildContext context) {
    return div(classes: 'navbar-menu${isActive ? ' is-active' : ''}', [
      div(classes: 'navbar-start', items),
      div(classes: 'navbar-end', endItems),
    ]);
  }
}

class BulmaNavbarItem extends StatelessComponent {
  const BulmaNavbarItem({required this.child, this.href, super.key})
    : items = null;

  const BulmaNavbarItem.dropdown({
    required this.child,
    required this.items,
    super.key,
  }) : href = null;

  final Component child;
  final String? href;
  final List<Component>? items;

  @override
  Component build(BuildContext context) {
    var classes = 'navbar-item';
    if (items == null) {
      return href == null
          ? div(classes: classes, [child])
          : a(href: href!, classes: classes, [child]);
    } else {
      return div(classes: '$classes has-dropdown is-hoverable', [
        a(href: '', classes: 'navbar-link', [child]),
        div(classes: 'navbar-dropdown', items!),
      ]);
    }
  }
}

class BulmaNavbarDivider extends StatelessComponent {
  const BulmaNavbarDivider({super.key});

  @override
  Component build(BuildContext context) {
    return hr(classes: 'navbar-divider');
  }
}

class BulmaNavbarPosition extends StatelessComponent {
  const BulmaNavbarPosition.end(this.children, {super.key}) : isEnd = true;
  const BulmaNavbarPosition.start(this.children, {super.key}) : isEnd = false;

  final List<Component> children;
  final bool isEnd;

  @override
  Component build(BuildContext context) {
    return div(classes: 'navbar-end', children);
  }
}
