import 'package:jaspr/jaspr.dart';

/// Notification types matching Bulma's color scheme
enum NotificationType { info, success, warning, danger }

/// Internal wrapper for notifications with unique IDs
class _NotificationItem {
  final int id;
  final Component notification;
  final Duration? duration;

  const _NotificationItem({
    required this.id,
    required this.notification,
    required this.duration,
  });
}

/// Notification Messenger - similar to Flutter's ScaffoldMessenger
class NotificationMessenger extends InheritedComponent {
  const NotificationMessenger({
    required this.state,
    required super.child,
    super.key,
  });

  final NotificationMessengerState state;

  /// Get the NotificationMessenger from context
  static NotificationMessengerState of(BuildContext context) {
    final messenger = context
        .dependOnInheritedComponentOfExactType<NotificationMessenger>();
    assert(messenger != null, 'No NotificationMessenger found in context');
    return messenger!.state;
  }

  @override
  bool updateShouldNotify(covariant NotificationMessenger oldComponent) {
    return false;
  }
}

/// Stateful wrapper for NotificationMessenger
class NotificationMessengerProvider extends StatefulComponent {
  const NotificationMessengerProvider({required this.child, super.key});

  final Component child;

  @override
  State<NotificationMessengerProvider> createState() =>
      NotificationMessengerState();
}

/// State for NotificationMessenger
class NotificationMessengerState extends State<NotificationMessengerProvider> {
  final List<_NotificationItem> _notifications = [];
  int _nextId = 0;

  /// Show a notification message
  void showNotification(
    Component notification, {
    Duration? duration = const Duration(seconds: 5),
  }) {
    final id = _nextId++;
    final item = _NotificationItem(
      id: id,
      notification: notification,
      duration: duration,
    );

    setState(() {
      _notifications.add(item);
    });

    // Auto-hide notification after duration
    if (duration != null) {
      Future.delayed(duration, () {
        if (mounted) {
          _removeNotification(id);
        }
      });
    }
  }

  /// Remove a specific notification by id
  void _removeNotification(int id) {
    setState(() {
      _notifications.removeWhere((item) => item.id == id);
    });
  }

  @override
  Component build(BuildContext context) {
    return NotificationMessenger(
      state: this,
      child: div(
        attributes: {'style': 'position: relative'},
        [
          component.child,

          // Notification overlay - stack all notifications
          if (_notifications.isNotEmpty)
            div(
              attributes: {
                'style':
                    'position: fixed; top: 1rem; right: 1rem; z-index: 1000; '
                    'display: flex; flex-direction: column; gap: 0.5rem; max-width: 400px;',
              },
              [for (final item in _notifications) item.notification],
            ),
        ],
      ),
    );
  }
}

/// Internal notification widget for rendering
class BulmaNotification extends StatelessComponent {
  const BulmaNotification({
    this.title,
    required this.type,
    required this.child,
  });

  final Component? title;
  final Component child;
  final NotificationType type;

  // /// Show a simple message notification
  // void showMessage(
  //   String message, {
  //   NotificationType type = NotificationType.info,
  //   String? title,
  //   Duration duration = const Duration(seconds: 5),
  // }) {
  //   showNotification(
  //     NotificationMessage(
  //       message: message,
  //       type: type,
  //       title: title,
  //       duration: duration,
  //     ),
  //   );
  // }

  /// Show an error notification
  factory BulmaNotification.error(String message, {String? title}) {
    return BulmaNotification(
      child: text(message),
      type: NotificationType.danger,
      title: text(title ?? 'Error'),
    );
  }

  factory BulmaNotification.success(String message, {String? title}) {
    return BulmaNotification(
      child: text(message),
      type: NotificationType.success,
      title: text(title ?? 'Success'),
    );
  }

  String get _typeClass {
    switch (type) {
      case NotificationType.info:
        return 'is-info';
      case NotificationType.success:
        return 'is-success';
      case NotificationType.warning:
        return 'is-warning';
      case NotificationType.danger:
        return 'is-danger';
    }
  }

  String get _iconClass {
    switch (type) {
      case NotificationType.info:
        return 'fa-info-circle';
      case NotificationType.success:
        return 'fa-check-circle';
      case NotificationType.warning:
        return 'fa-exclamation-triangle';
      case NotificationType.danger:
        return 'fa-exclamation-circle';
    }
  }

  @override
  Component build(BuildContext context) {
    return div(
      classes: 'notification $_typeClass is-light notification-toast',
      attributes: {
        'style':
            'position: fixed; top: 20px; right: 20px; z-index: 9999; min-width: 300px; max-width: 500px; '
            'box-shadow: 0 0.5em 1em -0.125em rgba(10,10,10,.1), 0 0px 0 1px rgba(10,10,10,.02); '
            'animation: slideIn 0.3s ease-out;',
      },
      [
        div(classes: 'content', [
          if (title != null) ...[
            p(classes: 'is-size-6 has-text-weight-semibold mb-2', [
              span(classes: 'icon-text', [
                span(classes: 'icon', [i(classes: 'fas $_iconClass', [])]),
                span([title!]),
              ]),
            ]),
          ],
          p(classes: 'is-size-6 mb-0', [child]),
        ]),
      ],
    );
  }
}
