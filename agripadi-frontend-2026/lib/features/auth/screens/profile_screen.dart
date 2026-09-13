import 'package:agripadi/features/auth/models/auth_user.dart';
import 'package:agripadi/features/auth/providers/auth_provider.dart';
import 'package:agripadi/features/auth/screens/change_password_screen.dart';
import 'package:agripadi/features/auth/screens/login_screen.dart';
import 'package:agripadi/features/chats/providers/chat_provider.dart';
import 'package:agripadi/features/conversation/providers/conversation_provider.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

class ProfileScreen extends StatefulWidget {
  const ProfileScreen({super.key});

  @override
  State<ProfileScreen> createState() => _ProfileScreenState();
}

class _ProfileScreenState extends State<ProfileScreen> {
  final _formKey = GlobalKey<FormState>();
  final _nameController = TextEditingController();
  final _emailController = TextEditingController();
  final _phoneController = TextEditingController();
  bool _isEditing = false;
  bool _initialized = false;
  String? _lastSyncedProfileSignature;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();

    if (_initialized) {
      return;
    }

    _initialized = true;
    WidgetsBinding.instance.addPostFrameCallback((_) async {
      if (!mounted) {
        return;
      }

      final authProvider = context.read<AuthProvider>();
      if (authProvider.user == null) {
        await authProvider.loadProfile();
      }

      if (!mounted) {
        return;
      }

      _syncControllers();
    });
  }

  @override
  void dispose() {
    _nameController.dispose();
    _emailController.dispose();
    _phoneController.dispose();
    super.dispose();
  }

  void _syncControllers() {
    if (!mounted) {
      return;
    }

    final user = context.read<AuthProvider>().user;
    if (user == null) {
      return;
    }

    _nameController.text = user.name;
    _emailController.text = user.email;
    _phoneController.text = user.phoneNumber;
    _lastSyncedProfileSignature = _profileSignature(user);
  }

  void _syncControllersAfterBuild(AuthUser user) {
    final signature = _profileSignature(user);
    if (_isEditing || _lastSyncedProfileSignature == signature) {
      return;
    }

    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted || _isEditing) {
        return;
      }
      _syncControllers();
    });
  }

  Future<void> _saveProfile() async {
    FocusScope.of(context).unfocus();

    if (!_formKey.currentState!.validate()) {
      return;
    }

    final success = await context.read<AuthProvider>().updateProfile(
      name: _nameController.text,
      email: _emailController.text,
      phoneNumber: _phoneController.text,
    );

    if (!mounted || !success) {
      return;
    }

    _syncControllers();

    setState(() {
      _isEditing = false;
    });

    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(content: Text("Profil berhasil diperbarui")),
    );
  }

  Future<void> _logout() async {
    final confirm = await showDialog<bool>(
      context: context,
      builder: (_) {
        return AlertDialog(
          title: const Text("Logout"),
          content: const Text("Keluar dari akun AgriPadi?"),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(context, false),
              child: const Text("Batal"),
            ),
            FilledButton(
              onPressed: () => Navigator.pop(context, true),
              child: const Text("Logout"),
            ),
          ],
        );
      },
    );

    if (confirm != true || !mounted) {
      return;
    }

    await context.read<AuthProvider>().logout();

    if (!mounted) {
      return;
    }

    context.read<ChatProvider>().clearChat();
    context.read<ConversationProvider>().clear();

    Navigator.pushAndRemoveUntil(
      context,
      MaterialPageRoute(builder: (_) => const LoginScreen()),
      (_) => false,
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: const Color(0xFFF7FAF7),
      appBar: AppBar(
        title: const Text("Profil"),
        actions: [
          IconButton(
            tooltip: _isEditing ? "Batal edit" : "Edit profil",
            onPressed: () {
              context.read<AuthProvider>().clearError();
              setState(() {
                _isEditing = !_isEditing;
                if (!_isEditing) {
                  _syncControllers();
                }
              });
            },
            icon: Icon(_isEditing ? Icons.close : Icons.edit_outlined),
          ),
        ],
      ),
      body: SafeArea(
        child: Consumer<AuthProvider>(
          builder: (_, provider, _) {
            if (provider.isLoading && provider.user == null) {
              return const Center(child: CircularProgressIndicator());
            }

            final user = provider.user;
            if (user == null) {
              return Center(
                child: FilledButton.icon(
                  onPressed: provider.loadProfile,
                  icon: const Icon(Icons.refresh),
                  label: const Text("Muat profil"),
                ),
              );
            }

            _syncControllersAfterBuild(user);

            return SingleChildScrollView(
              padding: const EdgeInsets.all(20),
              child: Center(
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 560),
                  child: Form(
                    key: _formKey,
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        Center(
                          child: CircleAvatar(
                            radius: 42,
                            backgroundColor: Colors.green.shade100,
                            child: Text(
                              _profileInitial(user.name),
                              style: TextStyle(
                                color: Colors.green.shade800,
                                fontSize: 30,
                                fontWeight: FontWeight.w800,
                              ),
                            ),
                          ),
                        ),
                        const SizedBox(height: 14),
                        Text(
                          user.name,
                          textAlign: TextAlign.center,
                          style: const TextStyle(
                            fontSize: 24,
                            fontWeight: FontWeight.w800,
                          ),
                        ),
                        const SizedBox(height: 4),
                        Text(
                          user.phoneNumber,
                          textAlign: TextAlign.center,
                          style: TextStyle(color: Colors.grey.shade700),
                        ),
                        const SizedBox(height: 28),
                        TextFormField(
                          controller: _nameController,
                          enabled: _isEditing,
                          decoration: const InputDecoration(
                            labelText: "Nama",
                            prefixIcon: Icon(Icons.person_outline),
                            border: OutlineInputBorder(),
                          ),
                          validator: (value) {
                            final name = value?.trim() ?? "";
                            if (name.isEmpty) {
                              return "Nama wajib diisi";
                            }
                            if (name.length > 32) {
                              return "Nama maksimal 32 karakter";
                            }
                            return null;
                          },
                        ),
                        const SizedBox(height: 14),
                        TextFormField(
                          controller: _emailController,
                          enabled: _isEditing,
                          keyboardType: TextInputType.emailAddress,
                          decoration: const InputDecoration(
                            labelText: "Email",
                            prefixIcon: Icon(Icons.mail_outline),
                            border: OutlineInputBorder(),
                          ),
                          validator: (value) {
                            final email = value?.trim() ?? "";
                            if (email.isEmpty) {
                              return "Email wajib diisi";
                            }
                            if (!email.contains("@")) {
                              return "Format email tidak valid";
                            }
                            return null;
                          },
                        ),
                        const SizedBox(height: 14),
                        TextFormField(
                          controller: _phoneController,
                          enabled: _isEditing,
                          keyboardType: TextInputType.phone,
                          decoration: const InputDecoration(
                            labelText: "Nomor telepon",
                            prefixIcon: Icon(Icons.phone_outlined),
                            border: OutlineInputBorder(),
                          ),
                          validator: (value) {
                            final phoneNumber = value?.trim() ?? "";
                            if (phoneNumber.isEmpty) {
                              return "Nomor telepon wajib diisi";
                            }
                            if (phoneNumber.length > 32) {
                              return "Nomor telepon maksimal 32 karakter";
                            }
                            return null;
                          },
                        ),
                        if (provider.errorMessage != null) ...[
                          const SizedBox(height: 14),
                          _ErrorBanner(message: provider.errorMessage!),
                        ],
                        const SizedBox(height: 22),
                        if (_isEditing)
                          FilledButton.icon(
                            onPressed: provider.isLoading ? null : _saveProfile,
                            icon: provider.isLoading
                                ? const SizedBox(
                                    width: 18,
                                    height: 18,
                                    child: CircularProgressIndicator(
                                      strokeWidth: 2,
                                    ),
                                  )
                                : const Icon(Icons.save_outlined),
                            label: const Text("Simpan Perubahan"),
                          ),
                        const SizedBox(height: 12),

                        Card(
                          child: ListTile(
                            leading: const Icon(Icons.lock_outline),
                            title: const Text("Ubah Password"),
                            subtitle: const Text("Perbarui password akun"),
                            trailing: const Icon(Icons.chevron_right),
                            onTap: () {
                              Navigator.push(
                                context,
                                MaterialPageRoute(
                                  builder: (_) => const ChangePasswordScreen(),
                                ),
                              );
                            },
                          ),
                        ),

                        const SizedBox(height: 12),

                        OutlinedButton.icon(
                          onPressed: provider.isLoading ? null : _logout,
                          icon: const Icon(Icons.logout),
                          label: const Text("Logout"),
                          style: OutlinedButton.styleFrom(
                            foregroundColor: Colors.red.shade700,
                            side: BorderSide(color: Colors.red.shade200),
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              ),
            );
          },
        ),
      ),
    );
  }
}

class _ErrorBanner extends StatelessWidget {
  final String message;

  const _ErrorBanner({required this.message});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: Colors.red.shade50,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: Colors.red.shade100),
      ),
      child: Row(
        children: [
          Icon(Icons.error_outline, color: Colors.red.shade700),
          const SizedBox(width: 10),
          Expanded(
            child: Text(message, style: TextStyle(color: Colors.red.shade700)),
          ),
        ],
      ),
    );
  }
}

String _profileSignature(AuthUser user) {
  return "${user.id}|${user.name}|${user.email}|${user.phoneNumber}";
}

String _profileInitial(String value) {
  final trimmed = value.trim();
  if (trimmed.isEmpty) {
    return "A";
  }

  return String.fromCharCode(trimmed.runes.first).toUpperCase();
}
