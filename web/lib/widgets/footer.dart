import 'package:jaspr/jaspr.dart';

class BulmaFooter extends StatelessComponent {
  const BulmaFooter({super.key});

  @override
  Component build(BuildContext context) {
    return footer(classes: 'footer', [
      div(classes: 'content has-text-centered', [
        p([
          strong([text('Artifact Server')]),
          text(' by '),
          a(href: 'https://hub.docker.com/r/davbfr/artifact', [
            text('davbfr/artifact'),
          ]),
          text('. The source code is licensed under '),
          a(href: 'https://opensource.org/license/apache-2-0', [
            text('APACHE 2.0'),
          ]),
          text('.'),
        ]),
      ]),
    ]);
  }
}
