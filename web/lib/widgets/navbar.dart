import 'package:jaspr/dom.dart';
import 'package:jaspr/jaspr.dart';

import '../bulma/bulma.dart';
import 'logo.dart';

class NavBar extends StatelessComponent {
  const NavBar({
    required this.isAuthenticated,
    required this.onAuthToggle,
    required this.onRefresh,
    this.altPressed = false,
    this.showTitleAndRefresh = false,
    this.sessionLabel,
    super.key,
  });
  final bool isAuthenticated;
  final void Function(bool value) onAuthToggle;
  final void Function() onRefresh;
  final bool altPressed;
  final bool showTitleAndRefresh;

  /// Who is signed in, decoded from the session token for display only. Null
  /// for the static token, which carries no identity.
  final String? sessionLabel;

  @override
  Component build(BuildContext context) {
    return BulmaNavBar([
      BulmaNavbarBrand(
        children: [
          if (showTitleAndRefresh) ...[
            const BulmaNavbarItem(child: Logo()),
            const BulmaNavbarItem(child: Component.text('Artifact Server')),
          ],
        ],
      ),

      BulmaNavbarPosition.end([
        if (isAuthenticated && sessionLabel != null)
          BulmaNavbarItem(
            child: span(classes: 'tag is-light is-medium', [
              const span(classes: 'icon', [i(classes: 'fas fa-user', [])]),
              span([Component.text(sessionLabel!)]),
            ]),
          ),

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

        if (showTitleAndRefresh)
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
