import 'package:deepyr/deepyr.dart';
import 'package:jaspr/jaspr.dart';

class MyLoading extends StatelessComponent {
  const MyLoading({super.key, this.message = 'Loading...'});

  final String message;

  @override
  Component build(BuildContext context) {
    return div([
      Loading(style: [Loading.ring, Loading.xl]),
      text('Loading...'),
    ]);
  }
}
