import 'package:agripadi/core/storage/secure_storage.dart';
import 'package:agripadi/features/auth/screens/login_screen.dart';
import 'package:flutter/material.dart';

class AuthGuard {
  AuthGuard._();

  static final navigatorKey =
      GlobalKey<NavigatorState>();

  static Future<void> handleUnauthorized() async {
    await TokenStorage.clear();

    final context =
        navigatorKey.currentContext;

    if (context == null || !context.mounted) {
      return;
    }

    Navigator.pushAndRemoveUntil(
      context,
      MaterialPageRoute(
        builder: (_) =>
            const LoginScreen(),
      ),
      (_) => false,
    );
  }
}