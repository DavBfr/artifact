import 'package:jaspr/jaspr.dart';
import 'package:universal_web/web.dart' as web;

class AuthDialog extends StatefulComponent {
  final void Function(String token) onLogin;
  final VoidCallback onCancel;

  const AuthDialog({required this.onLogin, required this.onCancel, super.key});

  @override
  State<AuthDialog> createState() => _AuthDialogState();
}

class _AuthDialogState extends State<AuthDialog> {
  String _token = '';

  @override
  void initState() {
    super.initState();
    Future.delayed(Duration(milliseconds: 100), _focusTokenInput);
  }

  void _focusTokenInput() {
    final input = web.document.getElementById('token-input');
    if (input != null) {
      (input as web.HTMLInputElement).focus();
    }
  }

  void _handleSubmit() {
    if (_token.trim().isNotEmpty) {
      component.onLogin(_token.trim());
      setState(() {
        _token = '';
      });
    }
  }

  void _handleKeyPress(web.KeyboardEvent event) {
    if (event.key == 'Enter') {
      event.preventDefault();
      _handleSubmit();
    } else if (event.key == 'Escape') {
      event.preventDefault();
      component.onCancel();
    }
  }

  @override
  Component build(BuildContext context) {
    return div(classes: 'modal is-active', [
      div(
        classes: 'modal-background',
        events: {'click': (_) => component.onCancel()},
        [],
      ),
      div(
        classes: 'modal-card',
        attributes: {'style': 'width: 90%; max-width: 540px;'},
        [
          // Header
          header(classes: 'modal-card-head', [
            p(classes: 'modal-card-title', [
              span(classes: 'icon-text', [
                span(classes: 'icon has-text-primary', [
                  i(classes: 'fas fa-key', []),
                ]),
                span([text('API Authentication')]),
              ]),
            ]),
            button(
              classes: 'delete',
              attributes: {'aria-label': 'close'},
              onClick: component.onCancel,
              [],
            ),
          ]),

          // Body
          section(classes: 'modal-card-body', [
            div(classes: 'content', [
              p(classes: 'has-text-grey', [
                text(
                  'Enter your API authentication token to upload and manage artifacts.',
                ),
              ]),
            ]),
            div(classes: 'field', [
              label(classes: 'label', [text('API Token')]),
              div(classes: 'control has-icons-left', [
                input(
                  type: InputType.password,
                  classes: 'input is-medium',
                  id: 'token-input',
                  attributes: {
                    'placeholder': 'Enter your API token',
                    'autocomplete': 'off',
                  },
                  events: {
                    'input': (event) {
                      final target = event.target as web.HTMLInputElement;
                      setState(() {
                        _token = target.value;
                      });
                    },
                    'keydown': (event) {
                      _handleKeyPress(event as web.KeyboardEvent);
                    },
                  },
                ),
                span(classes: 'icon is-left', [i(classes: 'fas fa-lock', [])]),
              ]),
              if (_token.isNotEmpty)
                p(classes: 'help is-success', [
                  text('Token entered (${_token.length} characters)'),
                ]),
            ]),

            div(classes: 'notification is-info is-light mt-4', [
              p(classes: 'is-size-7', [
                strong([text('Note: ')]),
                text(
                  'The token is securely stored in your browser\'s local storage and will persist across sessions.',
                ),
              ]),
            ]),
          ]),

          // Footer
          footer(classes: 'modal-card-foot', [
            button(
              classes: 'button is-primary',
              onClick: _token.trim().isNotEmpty ? () => _handleSubmit() : null,
              attributes: _token.trim().isEmpty ? {'disabled': 'true'} : {},
              [
                span(classes: 'icon', [i(classes: 'fas fa-sign-in-alt', [])]),
                span([text('Login')]),
              ],
            ),
            button(classes: 'button', onClick: component.onCancel, [
              text('Cancel'),
            ]),
          ]),
        ],
      ),
    ]);
  }
}
