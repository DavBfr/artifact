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
            div([], attributes: {'class': 'spinner'}),
            // message under spinner
            div([text(message)], attributes: {'class': 'loading-message'}),
          ],
          attributes: {'class': 'loading-box'},
        ),
      ],
      attributes: {'class': 'loading-wrapper'},
    );
  }
}
