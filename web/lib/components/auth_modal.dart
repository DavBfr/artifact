import 'package:jaspr/jaspr.dart';

class AuthModal extends StatefulComponent {
  final void Function(String) onLogin;

  const AuthModal({required this.onLogin, super.key});

  @override
  State<AuthModal> createState() => _AuthModalState();
}

class _AuthModalState extends State<AuthModal> {
  @override
  void initState() {
    super.initState();
    _setupLoginHandler();
  }

  void _setupLoginHandler() {
    Future.delayed(Duration.zero, () {
      // final loginBtn = html.document.querySelector(
      //   '.modal-footer .btn-primary',
      // );
      // loginBtn?.onClick.listen((event) {
      //   _handleLogin();
      // });
    });
  }

  void _handleLogin() {
    // final tokenInput =
    //     html.document.getElementById('auth-token') as html.InputElement?;
    // final token = tokenInput?.value?.trim() ?? '';

    // if (token.isEmpty) {
    //   _showNotification('Please enter a token', 'error');
    //   return;
    // }

    // // Close modal using Bootstrap's API
    // final modal = html.document.getElementById('authModal');
    // if (modal != null) {
    //   try {
    //     final dynamic bootstrap = html.window as dynamic;
    //     final dynamic modalInstance = bootstrap.bootstrap?.Modal?.getInstance(
    //       modal,
    //     );
    //     modalInstance?.hide();
    //   } catch (e) {
    //     print('Failed to close modal: $e');
    //   }
    // }

    // // Clear input
    // if (tokenInput != null) {
    //   tokenInput.value = '';
    // }

    // // Call the login callback
    // component.onLogin(token);
  }

  void _showNotification(String message, String type) {
    // final toast = html.document.getElementById('notification-toast');
    // final toastBody = toast?.querySelector('.toast-body');
    // final toastIcon = toast?.querySelector('.bi');

    // if (toastBody != null) {
    //   toastBody.text = message;
    // }

    // if (toastIcon != null) {
    //   toastIcon.className = type == 'success'
    //       ? 'bi bi-check-circle text-success me-2'
    //       : type == 'error'
    //       ? 'bi bi-exclamation-circle text-danger me-2'
    //       : 'bi bi-info-circle text-primary me-2';
    // }

    // if (toast != null) {
    //   try {
    //     final dynamic bootstrap = html.window as dynamic;
    //     bootstrap.bootstrap?.Toast(toast)?.show();
    //   } catch (e) {
    //     print('Failed to show toast: $e');
    //   }
    // }
  }

  @override
  Component build(BuildContext context) {
    return div(
      classes: 'modal fade',
      id: 'authModal',
      attributes: {
        'tabindex': '-1',
        'aria-labelledby': 'authModalLabel',
        'aria-hidden': 'true',
      },
      [
        div(classes: 'modal-dialog', [
          div(classes: 'modal-content', [
            div(classes: 'modal-header', [
              h5(classes: 'modal-title', id: 'authModalLabel', [
                text('Authentication'),
              ]),
              button(
                type: ButtonType.button,
                classes: 'btn-close',
                attributes: {'data-bs-dismiss': 'modal', 'aria-label': 'Close'},
                [],
              ),
            ]),
            div(classes: 'modal-body', [
              div(classes: 'mb-3', [
                label(
                  classes: 'form-label',
                  attributes: {'for': 'auth-token'},
                  [text('API Token')],
                ),
                input(
                  type: InputType.password,
                  classes: 'form-control',
                  id: 'auth-token',
                  attributes: {'placeholder': 'Enter your API token'},
                ),
              ]),
            ]),
            div(classes: 'modal-footer', [
              button(
                type: ButtonType.button,
                classes: 'btn btn-secondary',
                attributes: {'data-bs-dismiss': 'modal'},
                [text('Cancel')],
              ),
              button(type: ButtonType.button, classes: 'btn btn-primary', [
                text('Login'),
              ]),
            ]),
          ]),
        ]),
      ],
    );
  }
}
