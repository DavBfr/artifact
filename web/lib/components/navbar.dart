import 'package:artifact_web/components/bulma_navbar.dart';
import 'package:artifact_web/components/logo.dart';
import 'package:jaspr/jaspr.dart' hide Spacing;

import 'bulma_button.dart';

class NavBar extends StatelessComponent {
  final bool isAuthenticated;
  final void Function(bool value) onAuthToggle;
  final void Function() onRefresh;
  final bool altPressed;

  const NavBar({
    required this.isAuthenticated,
    required this.onAuthToggle,
    required this.onRefresh,
    this.altPressed = false,
    super.key,
  });

  @override
  Component build(BuildContext context) {
    return BulmaNavBar([
      BulmaNavbarBrand(
        children: [
          BulmaNavbarItem(child: Logo()),
          BulmaNavbarItem(child: text('Artifact Server')),
        ],
      ),

      BulmaNavbarPosition.end([
        if (isAuthenticated)
          BulmaNavbarItem(
            child: BulmaButton(
              child: IconLabel(icon: 'lock', label: 'Logout'),
              onPressed: () {
                onAuthToggle(false);
              },
            ),
          )
        else if (altPressed)
          BulmaNavbarItem(
            child: BulmaButton(
              child: IconLabel(icon: 'unlock', label: 'Login'),
              onPressed: () {
                onAuthToggle(true);
              },
            ),
          ),

        BulmaNavbarItem(
          child: BulmaButton(
            child: IconLabel(icon: 'refresh', label: 'Refresh'),
            onPressed: () {
              onRefresh();
            },
          ),
        ),
      ]),
    ]);
  }
}
