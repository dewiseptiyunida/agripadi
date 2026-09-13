import 'dart:convert';

import 'package:agripadi/core/network/api_client.dart';
import 'package:agripadi/core/storage/secure_storage.dart';
import 'package:agripadi/features/auth/models/auth_session.dart';
import 'package:agripadi/features/auth/models/auth_user.dart';

class AuthService {
  final ApiClient _apiClient = ApiClient.instance;

  Future<AuthSession> register({
    required String name,
    required String email,
    required String phoneNumber,
    required String password,
    required String confirmPassword,
  }) async {
    final response = await _apiClient.post(
      "/auth/register",
      body: {
        "name": name,
        "email": email,
        "phone_number": phoneNumber,
        "password": password,
        "confirm_password": confirmPassword,
      },
    );

    final session = AuthSession.fromJson(
      Map<String, dynamic>.from(response["data"] ?? response),
    );

    await _saveSession(session);

    return session;
  }

  Future<AuthSession> login({
    required String phoneNumber,
    required String password,
  }) async {
    final response = await _apiClient.post(
      "/auth/login",
      body: {"phone_number": phoneNumber, "password": password},
    );

    final session = AuthSession.fromJson(
      Map<String, dynamic>.from(response["data"] ?? response),
    );

    await _saveSession(session);

    return session;
  }

  Future<AuthUser> getProfile() async {
    final response = await _apiClient.get("/profile", withAuth: true);

    return AuthUser.fromJson(
      Map<String, dynamic>.from(response["data"] ?? response),
    );
  }

  Future<AuthUser> updateProfile({
    required String name,
    required String email,
    required String phoneNumber,
  }) async {
    final response = await _apiClient.patch(
      "/profile",
      withAuth: true,
      body: {"name": name, "email": email, "phone_number": phoneNumber},
    );

    return AuthUser.fromJson(
      Map<String, dynamic>.from(response["data"] ?? response),
    );
  }

  Future<void> changePassword({
    required String currentPassword,
    required String newPassword,
    required String confirmPassword,
  }) async {
    await _apiClient.patch(
      "/profile/password",
      withAuth: true,
      body: {
        "current_password": currentPassword,
        "new_password": newPassword,
        "confirm_password": confirmPassword,
      },
    );
  }

  Future<void> logout() async {
    try {
      final response = await _apiClient.post("/auth/logout", withAuth: true);
      print("LOGOUT RESPONSE: ${jsonEncode(response)}");
    } catch (e) {
      // Logout lokal tetap harus berhasil meskipun server sedang tidak dapat
      // dijangkau atau token sudah tidak berlaku.
      print("LOGOUT API WARNING: $e");
    } finally {
      await TokenStorage.clear();
    }
  }

  Future<void> _saveSession(AuthSession session) async {
    if (session.token.isEmpty || session.user.id.isEmpty) {
      throw const ApiException("Session login tidak valid.");
    }

    await TokenStorage.saveTokens(
      accessToken: session.token,
      userId: session.user.id,
    );
  }
}
