import 'package:jaspr/jaspr.dart';
import 'package:universal_web/js_interop.dart';
import 'package:universal_web/web.dart';

class KeyListener extends StatefulComponent {
  const KeyListener(this.child, {this.onKeyDown, this.onKeyUp, super.key});

  final Component child;

  final void Function(KeyboardEvent event)? onKeyDown;
  final void Function(KeyboardEvent event)? onKeyUp;

  @override
  State<KeyListener> createState() {
    return KeyListenerState();
  }
}

class KeyListenerState extends State<KeyListener> {
  @override
  void initState() {
    super.initState();

    // Attach key event listeners
    if (component.onKeyDown != null) {
      window.addEventListener('keydown', _onKeyDown.toJS);
    }

    if (component.onKeyUp != null) {
      window.addEventListener('keyup', _onKeyUp.toJS);
    }
  }

  void _onKeyDown(KeyboardEvent event) {
    component.onKeyDown!(event);
  }

  void _onKeyUp(KeyboardEvent event) {
    component.onKeyUp!(event);
  }

  @override
  void dispose() {
    // Remove key event listeners
    if (component.onKeyDown != null) {
      window.removeEventListener('keydown', _onKeyDown.toJS);
    }
    if (component.onKeyUp != null) {
      window.removeEventListener('keyup', _onKeyUp.toJS);
    }
    super.dispose();
  }

  @override
  Component build(BuildContext context) {
    return component.child;
  }
}
