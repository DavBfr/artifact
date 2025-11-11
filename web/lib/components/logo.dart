import 'package:jaspr/jaspr.dart';

class Logo extends StatelessComponent {
  const Logo({super.key});

  @override
  Component build(BuildContext context) {
    return img(src: 'logo.svg', attributes: {'width': '38', 'height': '38'});
  }
}
