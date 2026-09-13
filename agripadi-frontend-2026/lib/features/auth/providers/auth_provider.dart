import 'package:agripadi/core/auth/auth_guard.dart';
import 'package:agripadi/core/network/api_client.dart';
import 'package:agripadi/core/storage/secure_storage.dart';
import 'package:agripadi/features/auth/models/auth_user.dart';
import 'package:agripadi/features/auth/services/auth_service.dart';
import 'package:flutter/material.dart';

class AuthProvider extends ChangeNotifier {
  final AuthService _authService = AuthService();

  bool _isLoading = false;
  String? _errorMessage;
  AuthUser? _user;

  bool get isLoading => _isLoading;
  String? get errorMessage => _errorMessage;
  AuthUser? get user => _user;

  void _setLoading(bool value) {
    _isLoading = value;
    notifyListeners();
  }

  void clearError() {
    _errorMessage = null;
    notifyListeners();
  }

  Future<bool> hasSession() {
    return TokenStorage.hasSession();
  }

  /// Memvalidasi sesi yang tersimpan tanpa membuang sesi saat server sedang
  /// tidak dapat dijangkau. Token hanya dihapus ketika backend benar-benar
  /// mengembalikan status tidak sah (401).
  Future<bool> restoreSession() async {
    final storedSession = await TokenStorage.hasSession();
    if (!storedSession) {
      return false;
    }

    try {
      _user = await _authService.getProfile();
      _errorMessage = null;
      notifyListeners();
      return true;
    } on UnauthorizedException {
      await TokenStorage.clear();
      _user = null;
      _errorMessage = null;
      notifyListeners();
      return false;
    } catch (e) {
      // Pertahankan sesi lokal ketika masalahnya jaringan/server sementara.
      // Halaman utama masih dapat terbuka dan akan menampilkan pesan koneksi
      // yang relevan ketika pengguna melakukan operasi daring.
      _errorMessage = e.toString();
      notifyListeners();
      return true;
    }
  }

  Future<void> loadProfile() async {
    try {
      _setLoading(true);
      _errorMessage = null;
      _user = await _authService.getProfile();
      notifyListeners();
    } on UnauthorizedException {
      await AuthGuard.handleUnauthorized();
    } catch (e) {
      _errorMessage = e.toString();
      notifyListeners();
    } finally {
      _setLoading(false);
    }
  }

  Future<bool> login({
    required String phoneNumber,
    required String password,
  }) async {
    try {
      _setLoading(true);
      _errorMessage = null;
      final session = await _authService.login(
        phoneNumber: phoneNumber.trim(),
        password: password,
      );
      _user = session.user;
      notifyListeners();
      return true;
    } catch (e) {
      _errorMessage = e.toString();
      notifyListeners();
      return false;
    } finally {
      _setLoading(false);
    }
  }

  Future<bool> register({
    required String name,
    required String email,
    required String phoneNumber,
    required String password,
    required String confirmPassword,
  }) async {
    try {
      _setLoading(true);
      _errorMessage = null;
      final session = await _authService.register(
        name: name.trim(),
        email: email.trim(),
        phoneNumber: phoneNumber.trim(),
        password: password,
        confirmPassword: confirmPassword,
      );
      _user = session.user;
      notifyListeners();
      return true;
    } catch (e) {
      _errorMessage = e.toString();
      notifyListeners();
      return false;
    } finally {
      _setLoading(false);
    }
  }

  Future<bool> updateProfile({
    required String name,
    required String email,
    required String phoneNumber,
  }) async {
    try {
      _setLoading(true);
      _errorMessage = null;
      _user = await _authService.updateProfile(
        name: name.trim(),
        email: email.trim(),
        phoneNumber: phoneNumber.trim(),
      );
      notifyListeners();
      return true;
    } on UnauthorizedException {
      await AuthGuard.handleUnauthorized();
      return false;
    } catch (e) {
      _errorMessage = e.toString();
      notifyListeners();
      return false;
    } finally {
      _setLoading(false);
    }
  }

  Future<bool> changePassword({
    required String currentPassword,
    required String newPassword,
    required String confirmPassword,
  }) async {
    try {
      _setLoading(true);
      _errorMessage = null;

      await _authService.changePassword(
        currentPassword: currentPassword,
        newPassword: newPassword,
        confirmPassword: confirmPassword,
      );

      notifyListeners();
      return true;
    } on UnauthorizedException {
      await AuthGuard.handleUnauthorized();
      return false;
    } catch (e) {
      _errorMessage = e.toString();
      notifyListeners();
      return false;
    } finally {
      _setLoading(false);
    }
  }

  Future<void> logout() async {
    _setLoading(true);
    try {
      await _authService.logout();
    } finally {
      // State autentikasi lokal harus selalu bersih setelah pengguna menekan
      // logout, termasuk ketika request logout server gagal.
      _user = null;
      _errorMessage = null;
      _setLoading(false);
    }
  }
}
