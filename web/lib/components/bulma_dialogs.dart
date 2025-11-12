import 'dart:async';

import 'package:jaspr/jaspr.dart';

/// Internal wrapper for dialogs with unique IDs
class _DialogItem<T> extends StatelessComponent {
  const _DialogItem({
    required this.id,
    required this.child,
    required this.onComplete,
    required this.isDismissible,
  });

  final int id;
  final Component child;
  final void Function([T? value]) onComplete;
  final bool isDismissible;

  @override
  Component build(BuildContext context) {
    return div(classes: 'modal is-active', [
      div(
        classes: 'modal-background',
        events: isDismissible ? {'click': (_) => onComplete()} : {},
        [],
      ),
      div(
        classes: 'modal-card',
        attributes: {'style': 'width: 90%; max-width: 540px;'},
        [child],
      ),
    ]);
  }
}

/// Dialog Manager - similar to Flutter's dialog system
class DialogManager extends InheritedComponent {
  const DialogManager({required this.state, required super.child, super.key});

  final DialogManagerState state;

  /// Get the DialogManager from context
  static DialogManagerState of(BuildContext context) {
    final manager = context
        .dependOnInheritedComponentOfExactType<DialogManager>();
    assert(manager != null, 'No DialogManager found in context');
    return manager!.state;
  }

  @override
  bool updateShouldNotify(covariant DialogManager oldComponent) {
    return false;
  }
}

/// Stateful wrapper for DialogManager
class DialogManagerProvider extends StatefulComponent {
  const DialogManagerProvider({required this.child, super.key});

  final Component child;

  @override
  State<DialogManagerProvider> createState() => DialogManagerState();
}

/// State for DialogManager
class DialogManagerState extends State<DialogManagerProvider> {
  final List<_DialogItem> _dialogs = [];
  int _nextId = 0;

  /// Show a dialog and return its ID
  Future<T?> showDialog<T>(
    Component Function(void Function([T? value]) onComplete) build, {
    bool isDismissible = true,
  }) async {
    final id = _nextId++;
    final completer = Completer<T?>();
    void onComplete([T? value]) {
      completer.complete(value);
      _hideDialog(id);
    }

    final child = build(onComplete);
    final item = _DialogItem(
      id: id,
      child: child,
      onComplete: onComplete,
      isDismissible: isDismissible,
    );
    setState(() {
      _dialogs.add(item);
    });
    return completer.future;
  }

  /// Hide a specific dialog by ID
  void _hideDialog(int id) {
    if (!mounted) return;
    setState(() {
      _dialogs.removeWhere((item) => item.id == id);
    });
  }

  @override
  Component build(BuildContext context) {
    return DialogManager(
      state: this,
      child: div(
        attributes: {'style': 'position: relative'},
        [component.child, ..._dialogs],
      ),
    );
  }
}

class AlertDialog extends StatelessComponent {
  const AlertDialog({
    this.title,
    required this.content,
    this.actions = const [],
    super.key,
  });

  final Component? title;
  final List<Component> content;
  final List<Component> actions;

  @override
  Component build(BuildContext context) {
    return div([
      // Header
      if (title != null)
        header(classes: 'modal-card-head', [
          p(classes: 'modal-card-title', [title!]),
        ]),

      // Body
      section(classes: 'modal-card-body', [div(classes: 'content', content)]),

      // Footer
      if (actions.isNotEmpty) footer(classes: 'modal-card-foot', [...actions]),
    ]);
  }
}
