import 'auth_user.dart';

class AuthSession {
  final String token;
  final String tokenType;
  final DateTime? expiresAt;
  final AuthUser user;

  const AuthSession({
    required this.token,
    required this.tokenType,
    required this.expiresAt,
    required this.user,
  });

  factory AuthSession.fromJson(Map<String, dynamic> json) {
    return AuthSession(
      token: json["token"]?.toString() ?? "",
      tokenType: json["token_type"]?.toString() ?? "Bearer",
      expiresAt: DateTime.tryParse(json["expires_at"]?.toString() ?? ""),
      user: AuthUser.fromJson(
        Map<String, dynamic>.from(json["user"] as Map? ?? {}),
      ),
    );
  }
}
