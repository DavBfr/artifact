import 'package:jaspr/dom.dart';
import 'package:jaspr/jaspr.dart';

/// Bulma Card Component
/// See https://bulma.io/documentation/components/card/ for a detailed description
class BulmaCard extends StatelessComponent {
  const BulmaCard(
    this.content, {
    this.image,
    this.imageRatio = '4by3',
    this.avatar,
    this.title,
    this.subtitle,
    this.footer,
    super.key,
  });

  final Component? image;
  final String imageRatio;
  final Component? avatar;
  final Component? title;
  final Component? subtitle;
  final List<Component> content;
  final Component? footer;

  @override
  Component build(BuildContext context) {
    return div(classes: 'card', [
      // Card image section
      if (image != null)
        div(classes: 'card-image', [
          figure(classes: 'image is-$imageRatio', [image!]),
        ]),

      // Card content section
      div(classes: 'card-content', [
        // Media section with avatar and title
        if (avatar != null || title != null || subtitle != null)
          div(classes: 'media', [
            if (avatar != null)
              div(classes: 'media-left', [
                figure(classes: 'image is-48x48', [avatar!]),
              ]),
            if (title != null || subtitle != null)
              div(classes: 'media-content', [
                if (title != null) p(classes: 'title is-4', [title!]),
                if (subtitle != null) p(classes: 'subtitle is-6', [subtitle!]),
              ]),
          ]),

        // Content section
        div(classes: 'content', content),
      ]),

      // Footer section
      if (footer != null) div(classes: 'card-footer', [footer!]),
    ]);
  }
}

/// Card Footer Item Component
class BulmaCardFooterItem extends StatelessComponent {
  const BulmaCardFooterItem({
    required this.child,
    this.href,
    this.onClick,
    super.key,
  });

  final Component child;
  final String? href;
  final VoidCallback? onClick;

  @override
  Component build(BuildContext context) {
    return a(href: href ?? '', classes: 'card-footer-item', onClick: onClick, [
      child,
    ]);
  }
}
