import 'package:flutter/material.dart';

class AppColors {
  AppColors._();

  /// Background utama aplikasi chat.
  static const Color background = Color(0xFFF3F8F4);
  static const Color backgroundSoft = Color(0xFFEAF4ED);
  static const Color surface = Color(0xFFFFFFFF);
  static const Color surfaceSoft = Color(0xFFF7FBF8);

  /// Teks.
  static const Color textBlack = Color(0xFF17251D);
  static const Color textMuted = Color(0xFF66756B);
  static const Color textWhite = Color(0xFFFFFFFF);

  /// Brand / primary.
  static const Color primary = Color(0xFF1B5E3C);
  static const Color primaryDark = Color(0xFF0F3D2A);
  static const Color primaryLight = Color(0xFFDDF1E5);
  static const Color accent = Color(0xFF7AC143);

  /// CHAT BUBBLE.
  static const Color userBubble = Color(0xFF1B5E3C);
  static const Color aiBubble = Color(0xFFFFFFFF);
  static const Color chatCanvas = Color(0xFFF5FAF6);

  /// Border / stroke.
  static const Color stroke = Color(0xFFDCE7DF);
  static const Color strokeStrong = Color(0xFFBBD3C3);

  /// Semantic.
  static const Color red = Color(0xFFD32F2F);
  static const Color warning = Color(0xFFE9A23B);
  static const Color info = Color(0xFF1D7A8C);

  static LinearGradient get primaryGradient => const LinearGradient(
        begin: Alignment.topLeft,
        end: Alignment.bottomRight,
        colors: [primaryDark, primary],
      );
}
