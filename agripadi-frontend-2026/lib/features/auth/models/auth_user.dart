class AuthUser {
  final String id;
  final String name;
  final String email;
  final String phoneNumber;

  const AuthUser({
    required this.id,
    required this.name,
    required this.email,
    required this.phoneNumber,
  });

  factory AuthUser.fromJson(Map<String, dynamic> json) {
    return AuthUser(
      id: json["id"]?.toString() ?? "",
      name: json["name"]?.toString() ?? "",
      email: json["email"]?.toString() ?? "",
      phoneNumber: json["phone_number"]?.toString() ?? "",
    );
  }
}
