import 'dart:async';

import 'package:agripadi/features/auth/providers/auth_provider.dart';
import 'package:agripadi/features/auth/screens/login_screen.dart';
import 'package:agripadi/features/chats/screens/chat_screen.dart';
import 'package:agripadi/features/cnn/services/on_device_cnn_service.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

class SplashScreen extends StatefulWidget {
  const SplashScreen({super.key});

  @override
  State<SplashScreen> createState() => _SplashScreenState();
}

class _SplashScreenState extends State<SplashScreen> {
  @override
  void initState() {
    super.initState();

    _initialize();
  }

  Future<void> _initialize() async {
    try {
      await Future.delayed(const Duration(milliseconds: 900));

      if (!mounted) return;

      try {
        await OnDeviceCnnService.instance.preload();
      } catch (error) {
        debugPrint('CNN PRELOAD ERROR: $error');
      }

      if (!mounted) return;

      final hasSession = await context.read<AuthProvider>().restoreSession();

      if (!mounted) return;

      Navigator.pushReplacement(
        context,
        MaterialPageRoute(
          builder: (_) => hasSession ? const ChatScreen() : const LoginScreen(),
        ),
      );
    } catch (e) {
      debugPrint("SPLASH INITIALIZE ERROR: $e");

      if (!mounted) return;

      Navigator.pushReplacement(
        context,
        MaterialPageRoute(builder: (_) => const LoginScreen()),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: const Color(0xFF1B4332),

      body: SafeArea(
        child: Center(
          child: Padding(
            padding: const EdgeInsets.all(24),

            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,

              children: [
                // =============================
                // LOGO
                // =============================
                Container(
                  width: 140,
                  height: 140,

                  padding: const EdgeInsets.all(20),

                  decoration: BoxDecoration(
                    color: Colors.white,

                    borderRadius: BorderRadius.circular(28),

                    boxShadow: [
                      BoxShadow(
                        color: Colors.black.withOpacity(0.15),

                        blurRadius: 20,

                        offset: const Offset(0, 10),
                      ),
                    ],
                  ),

                  child: Image.asset(
                    "assets/icon/app_icon.png",

                    fit: BoxFit.contain,
                  ),
                ),

                const SizedBox(height: 28),

                // =============================
                // APP NAME
                // =============================
                const Text(
                  "AgriPadi Chat",

                  textAlign: TextAlign.center,

                  style: TextStyle(
                    color: Colors.white,

                    fontSize: 30,

                    fontWeight: FontWeight.bold,

                    letterSpacing: 1,
                  ),
                ),

                const SizedBox(height: 10),

                const Text(
                  "On-device CNN & Expert Chat",

                  textAlign: TextAlign.center,

                  style: TextStyle(
                    color: Colors.white70,

                    fontSize: 16,

                    letterSpacing: 0.5,
                  ),
                ),

                const SizedBox(height: 50),

                // =============================
                // LOADING
                // =============================
                const SizedBox(
                  width: 28,
                  height: 28,

                  child: CircularProgressIndicator(
                    color: Colors.white,
                    strokeWidth: 3,
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
