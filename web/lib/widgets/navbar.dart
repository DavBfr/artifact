import 'package:jaspr/jaspr.dart';

import '../bulma/bulma.dart';
import 'logo.dart';

class NavBar extends StatelessComponent {
  const NavBar({
    required this.isAuthenticated,
    required this.onAuthToggle,
    required this.onRefresh,
    this.altPressed = false,
    super.key,
  });
  final bool isAuthenticated;
  final void Function(bool value) onAuthToggle;
  final void Function() onRefresh;
  final bool altPressed;

  @override
  Component build(BuildContext context) {
    return BulmaNavBar([
      const BulmaNavbarBrand(
        children: [
          BulmaNavbarItem(child: Logo()),
          BulmaNavbarItem(child: Component.text('Artifact Server')),
        ],
      ),

      BulmaNavbarPosition.end([
        if (isAuthenticated)
          BulmaNavbarItem(
            child: BulmaButton(
              child: const IconLabel(icon: 'lock', label: 'Logout'),
              color: BulmaColor.info,
              onPressed: () {
                onAuthToggle(false);
              },
            ),
          )
        else if (altPressed)
          BulmaNavbarItem(
            child: BulmaButton(
              color: BulmaColor.info,
              child: const IconLabel(icon: 'unlock', label: 'Login'),
              onPressed: () {
                onAuthToggle(true);
              },
            ),
          ),

        BulmaNavbarItem(
          child: BulmaButton(
            child: const IconLabel(icon: 'refresh', label: 'Refresh'),
            onPressed: onRefresh,
          ),
        ),
      ]),
    ]);
  }
}
