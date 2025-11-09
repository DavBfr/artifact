import 'package:deepyr/deepyr.dart';
import 'package:jaspr/jaspr.dart' hide Spacing;

class NavBar extends StatelessComponent {
  final bool isAuthenticated;
  final void Function() onAuthToggle;
  final void Function() onRefresh;

  const NavBar({
    required this.isAuthenticated,
    required this.onAuthToggle,
    required this.onRefresh,
    super.key,
  });

  @override
  Component build(BuildContext context) {
    return Navbar(
      style: [BgUtil.base100, Effects.shadowSm],
      ariaLabel: 'Navbar with title',
      [
        NavbarStart([
          img(src: 'logo.svg', width: 38, height: 38),
          text(' Artifact Server'),
        ]),

        NavbarEnd([
          Button(
            [text('Login')],
            style: [Button.primary],
            onClick: (event) {
              onRefresh();
            },
          ),
          Button(
            [text('Refresh')],
            style: [Button.neutral],
            onClick: (event) {
              onRefresh();
            },
          ),
        ]),
      ],
    );

    return nav(classes: 'navbar navbar-expand-lg navbar-dark bg-dark', [
      div(classes: 'container', [
        span(classes: 'navbar-brand mb-0 h1', [
          img(src: 'logo.svg', width: 38, height: 38),
          text(' Artifact Server'),
        ]),
        div(classes: 'navbar-nav ms-auto', [
          button(
            classes: isAuthenticated
                ? 'btn btn-outline-success me-2'
                : 'btn btn-outline-light me-2',
            id: 'auth-btn',
            attributes: {
              'style': isAuthenticated
                  ? 'display: inline-block;'
                  : 'display: none;',
            },
            onClick: () => onAuthToggle(),
            [
              i(classes: 'bi bi-key', []),
              span(id: 'auth-btn-text', [
                text(isAuthenticated ? ' Logout' : ' Login'),
              ]),
            ],
          ),
          button(classes: 'btn btn-outline-light', onClick: () => onRefresh(), [
            i(classes: 'bi bi-arrow-clockwise refresh-btn', []),
            text(' Refresh'),
          ]),
        ]),
      ]),
    ]);
  }
}
