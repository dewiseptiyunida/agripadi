import 'package:flutter_secure_storage/flutter_secure_storage.dart';

class TokenStorage {
  TokenStorage._();

  // =========================================
  // STORAGE
  // =========================================

  static const FlutterSecureStorage _storage = FlutterSecureStorage();

  // =========================================
  // KEYS
  // =========================================

  static const String _accessTokenKey = "access_token";

  static const String _refreshTokenKey = "refresh_token";

  static const String _userIdKey = "user_id";

  // =========================================
  // SAVE TOKENS
  // =========================================

  static Future<void> saveTokens({
    required String accessToken,
    required String userId,
    String refreshToken = "",
  }) async {
    try {
      print("========================================");

      print("SAVE TOKENS");

      await _storage.write(key: _accessTokenKey, value: accessToken);

      if (refreshToken.isNotEmpty) {
        await _storage.write(key: _refreshTokenKey, value: refreshToken);
      } else {
        await _storage.delete(key: _refreshTokenKey);
      }

      await _storage.write(key: _userIdKey, value: userId);

      print("ACCESS TOKEN SAVED");

      print("REFRESH TOKEN SAVED: ${refreshToken.isNotEmpty}");

      print("USER ID SAVED: $userId");

      print("========================================");
    } catch (e) {
      print("SAVE TOKEN ERROR: $e");

      rethrow;
    }
  }

  // =========================================
  // SAVE USER ID
  // =========================================

  static Future<void> saveUserId(String userId) async {
    try {
      await _storage.write(key: _userIdKey, value: userId);

      print("USER ID SAVED: $userId");
    } catch (e) {
      print("SAVE USER ID ERROR: $e");

      rethrow;
    }
  }

  // =========================================
  // GET ACCESS TOKEN
  // =========================================

  static Future<String?> getAccessToken() async {
    try {
      final token = await _storage.read(key: _accessTokenKey);

      print("GET ACCESS TOKEN: ${token != null ? "FOUND" : "NULL"}");

      return token;
    } catch (e) {
      print("GET ACCESS TOKEN ERROR: $e");

      return null;
    }
  }

  // =========================================
  // GET REFRESH TOKEN
  // =========================================

  static Future<String?> getRefreshToken() async {
    try {
      final token = await _storage.read(key: _refreshTokenKey);

      print("GET REFRESH TOKEN: ${token != null ? "FOUND" : "NULL"}");

      return token;
    } catch (e) {
      print("GET REFRESH TOKEN ERROR: $e");

      return null;
    }
  }

  // =========================================
  // GET USER ID
  // =========================================

  static Future<String?> getUserId() async {
    try {
      final userId = await _storage.read(key: _userIdKey);

      print("GET USER ID: $userId");

      return userId;
    } catch (e) {
      print("GET USER ID ERROR: $e");

      return null;
    }
  }

  // =========================================
  // CHECK SESSION
  // =========================================

  static Future<bool> hasSession() async {
    try {
      final accessToken = await getAccessToken();

      final userId = await getUserId();

      final hasSession =
          accessToken != null &&
          accessToken.isNotEmpty &&
          userId != null &&
          userId.isNotEmpty;

      print("HAS SESSION: $hasSession");

      return hasSession;
    } catch (e) {
      print("HAS SESSION ERROR: $e");

      return false;
    }
  }

  // =========================================
  // REMOVE ACCESS TOKEN
  // =========================================

  static Future<void> removeAccessToken() async {
    try {
      await _storage.delete(key: _accessTokenKey);

      print("ACCESS TOKEN REMOVED");
    } catch (e) {
      print("REMOVE ACCESS TOKEN ERROR: $e");
    }
  }

  // =========================================
  // REMOVE REFRESH TOKEN
  // =========================================

  static Future<void> removeRefreshToken() async {
    try {
      await _storage.delete(key: _refreshTokenKey);

      print("REFRESH TOKEN REMOVED");
    } catch (e) {
      print("REMOVE REFRESH TOKEN ERROR: $e");
    }
  }

  // =========================================
  // CLEAR ALL
  // =========================================

  static Future<void> clear() async {
    try {
      print("========================================");

      print("CLEAR ALL TOKENS");

      await _storage.deleteAll();

      print("ALL TOKENS CLEARED");

      print("========================================");
    } catch (e) {
      print("CLEAR STORAGE ERROR: $e");

      rethrow;
    }
  }

  // =========================================
  // DEBUG SESSION
  // =========================================

  static Future<void> debugSession() async {
    final accessToken = await getAccessToken();

    final refreshToken = await getRefreshToken();

    final userId = await getUserId();

    print("========================================");

    print("SESSION DEBUG");

    print("ACCESS TOKEN: ${accessToken != null ? "EXISTS" : "NULL"}");

    print("REFRESH TOKEN: ${refreshToken != null ? "EXISTS" : "NULL"}");

    print("USER ID: $userId");

    print("========================================");
  }
}
