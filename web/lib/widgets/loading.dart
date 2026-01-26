import 'package:jaspr/dom.dart';
import 'package:jaspr/jaspr.dart';

class MyLoading extends StatelessComponent {
  const MyLoading({super.key, this.message = 'Loading...'});

  final String message;

  @override
  Component build(BuildContext context) {
    return div(
      [
        div(
          [
            // spinner element
            const div([], attributes: {'class': 'spinner'}),
            // message under spinner
            div(
              [Component.text(message)],
              attributes: const {'class': 'loading-message'},
            ),
          ],
          attributes: const {'class': 'loading-box'},
        ),
      ],
      attributes: const {'class': 'loading-wrapper'},
    );
  }
}
