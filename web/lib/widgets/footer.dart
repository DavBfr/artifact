import 'package:jaspr/dom.dart';
import 'package:jaspr/jaspr.dart';

class BulmaFooter extends StatelessComponent {
  const BulmaFooter({super.key});

  @override
  Component build(BuildContext context) {
    return const footer(classes: 'footer', [
      div(classes: 'content has-text-centered', [
        p([
          strong([Component.text('Artifact Server')]),
          Component.text(' by '),
          a(href: 'https://hub.docker.com/r/davbfr/artifact', [
            Component.text('davbfr/artifact'),
          ]),
          Component.text('. The source code is licensed under '),
          a(href: 'https://opensource.org/license/apache-2-0', [
            Component.text('APACHE 2.0'),
          ]),
          Component.text('.'),
        ]),
      ]),
    ]);
  }
}
