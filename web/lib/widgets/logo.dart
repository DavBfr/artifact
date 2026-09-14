import 'package:jaspr/dom.dart';
import 'package:jaspr/jaspr.dart';

class Logo extends StatelessComponent {
  const Logo({this.size = 38, super.key});

  final int size;

  @override
  Component build(BuildContext context) {
    return img(
      src: 'logo.svg',
      attributes: {'width': '$size', 'height': '$size'},
    );
  }
}
