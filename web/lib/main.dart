import 'package:jaspr/jaspr.dart';

import 'components/app.dart';

void main() {
  runApp(App());

  // runApp(
  //   Document(
  //     title: 'Artifact Server',
  //     styles: [
  //       css.import('styles.css'),
  //       // Special import rule to include to another css file.
  //       css.import('https://fonts.googleapis.com/css?family=Roboto'),
  //       // css.import(          'https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css',        ),
  //       // css.import(          'https://cdn.jsdelivr.net/npm/bootstrap-icons@1.10.0/font/bootstrap-icons.css',        ),
  //       // Each style rule takes a valid css selector and a set of styles.
  //       // Styles are defined using type-safe css bindings and can be freely chained and nested.
  //       css('html, body').styles(
  //         width: 100.percent,
  //         minHeight: 100.vh,
  //         padding: Padding.zero,
  //         margin: Margin.zero,
  //         fontFamily: const FontFamily.list([
  //           FontFamily('Roboto'),
  //           FontFamilies.sansSerif,
  //         ]),
  //         backgroundColor: Color('#f8f9fa'),
  //       ),
  //       css('h1').styles(margin: Margin.unset, fontSize: 4.rem),

  //       css('.navbar-brand').styles(fontWeight: FontWeight.w600),

  //       css('.upload-zone').styles(
  //         border: Border(
  //           width: 2.px,
  //           style: BorderStyle.dashed,
  //           color: Color('#dee2e6'),
  //         ),
  //         radius: BorderRadius.all(Radius.circular(8.px)),
  //         padding: Padding.all(40.px),
  //         textAlign: TextAlign.center,
  //         transition: Transition('all', duration: 0.3, curve: Curve.ease),
  //         backgroundColor: Color('white'),
  //       ),

  //       css('.upload-zone:hover').styles(
  //         border: Border(color: Color('#0d6efd')),
  //         backgroundColor: Color('#f8f9ff'),
  //       ),

  //       css('.upload-zone.dragover').styles(
  //         border: Border(color: Color('#0d6efd')),
  //         backgroundColor: Color('#e7f3ff'),
  //       ),

  //       css('.file-icon').styles(
  //         width: 40.px,
  //         height: 40.px,
  //         display: Display.flex,
  //         alignItems: AlignItems.center,
  //         justifyContent: JustifyContent.center,
  //         radius: BorderRadius.all(Radius.circular(8.px)),
  //         margin: Margin.only(right: 16.px),
  //       ),

  //       css(
  //         '.stats-card',
  //       ).styles(backgroundColor: Color('#667eea'), color: Color('white')),

  //       css('.refresh-btn').styles(
  //         transition: Transition('transform', duration: 0.3, curve: Curve.ease),
  //       ),

  //       css('.refresh-btn:hover').styles(transform: Transform.rotate(180.deg)),
  //     ],
  //     body: App(),
  //   ),
  // );
}
