import 'package:jaspr/jaspr.dart';

class Toast extends StatelessComponent {
  @override
  Component build(BuildContext context) {
    return div(classes: 'toast-container position-fixed bottom-0 end-0 p-3', [
      div(
        id: 'notification-toast',
        classes: 'toast',
        attributes: {
          'role': 'alert',
          'aria-live': 'assertive',
          'aria-atomic': 'true',
        },
        [
          div(classes: 'toast-header', [
            i(classes: 'bi bi-info-circle text-primary me-2', []),
            strong(classes: 'me-auto', [text('Notification')]),
            button(
              type: ButtonType.button,
              classes: 'btn-close',
              attributes: {'data-bs-dismiss': 'toast'},
              [],
            ),
          ]),
          div(classes: 'toast-body', [
            // Message will be inserted here by JavaScript
          ]),
        ],
      ),
    ]);
  }
}
