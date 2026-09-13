import 'package:agripadi/features/auth/providers/auth_provider.dart';
import 'package:agripadi/features/auth/screens/login_screen.dart';
import 'package:agripadi/features/chats/providers/chat_provider.dart';
import 'package:agripadi/features/conversation/providers/conversation_provider.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

class ChangePasswordScreen extends StatefulWidget {
  const ChangePasswordScreen({super.key});

  @override
  State<ChangePasswordScreen> createState() =>
      _ChangePasswordScreenState();
}

class _ChangePasswordScreenState
    extends State<ChangePasswordScreen> {
  final _formKey = GlobalKey<FormState>();

  final _currentPasswordController =
      TextEditingController();

  final _newPasswordController =
      TextEditingController();

  final _confirmPasswordController =
      TextEditingController();

  bool _obscureCurrent = true;
  bool _obscureNew = true;
  bool _obscureConfirm = true;

  @override
  void dispose() {
    _currentPasswordController.dispose();
    _newPasswordController.dispose();
    _confirmPasswordController.dispose();
    super.dispose();
  }

  Future<void> _changePassword() async {
    FocusScope.of(context).unfocus();

    if (!_formKey.currentState!.validate()) {
      return;
    }

    final provider =
        context.read<AuthProvider>();

    final success =
        await provider.changePassword(
      currentPassword:
          _currentPasswordController.text.trim(),
      newPassword:
          _newPasswordController.text.trim(),
      confirmPassword:
          _confirmPasswordController.text.trim(),
    );

    if (!mounted || !success) {
      return;
    }

    await showDialog(
      context: context,
      barrierDismissible: false,
      builder: (_) {
        return AlertDialog(
          title: const Text(
            "Password Berhasil Diubah",
          ),
          content: const Text(
            "Silakan login kembali menggunakan password baru Anda.",
          ),
          actions: [
            FilledButton(
              onPressed: () {
                Navigator.pop(context);
              },
              child: const Text("OK"),
            ),
          ],
        );
      },
    );

    if (!mounted) {
      return;
    }

    await provider.logout();

    if (!mounted) {
      return;
    }

    context.read<ChatProvider>().clearChat();
    context.read<ConversationProvider>().clear();

    Navigator.pushAndRemoveUntil(
      context,
      MaterialPageRoute(
        builder: (_) => const LoginScreen(),
      ),
      (_) => false,
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: const Color(
        0xFFF7FAF7,
      ),
      appBar: AppBar(
        title: const Text(
          "Ubah Password",
        ),
      ),
      body: SafeArea(
        child: Consumer<AuthProvider>(
          builder: (_, provider, _) {
            return SingleChildScrollView(
              padding:
                  const EdgeInsets.all(20),
              child: Center(
                child: ConstrainedBox(
                  constraints:
                      const BoxConstraints(
                    maxWidth: 560,
                  ),
                  child: Form(
                    key: _formKey,
                    child: Column(
                      crossAxisAlignment:
                          CrossAxisAlignment
                              .stretch,
                      children: [
                        Card(
                          child: Padding(
                            padding:
                                const EdgeInsets.all(
                              20,
                            ),
                            child: Column(
                              children: [
                                Icon(
                                  Icons.lock_reset,
                                  size: 48,
                                  color: Colors
                                      .green
                                      .shade700,
                                ),
                                const SizedBox(
                                  height: 12,
                                ),
                                const Text(
                                  "Perbarui Password Akun",
                                  textAlign:
                                      TextAlign
                                          .center,
                                  style:
                                      TextStyle(
                                    fontSize:
                                        20,
                                    fontWeight:
                                        FontWeight
                                            .w700,
                                  ),
                                ),
                                const SizedBox(
                                  height: 8,
                                ),
                                Text(
                                  "Gunakan password yang kuat dan mudah Anda ingat.",
                                  textAlign:
                                      TextAlign
                                          .center,
                                  style:
                                      TextStyle(
                                    color: Colors
                                        .grey
                                        .shade700,
                                  ),
                                ),
                              ],
                            ),
                          ),
                        ),

                        const SizedBox(
                          height: 20,
                        ),

                        TextFormField(
                          controller:
                              _currentPasswordController,
                          obscureText:
                              _obscureCurrent,
                          decoration:
                              InputDecoration(
                            labelText:
                                "Password Saat Ini",
                            prefixIcon:
                                const Icon(
                              Icons
                                  .lock_outline,
                            ),
                            suffixIcon:
                                IconButton(
                              onPressed:
                                  () {
                                setState(
                                  () {
                                    _obscureCurrent =
                                        !_obscureCurrent;
                                  },
                                );
                              },
                              icon: Icon(
                                _obscureCurrent
                                    ? Icons
                                        .visibility_off
                                    : Icons
                                        .visibility,
                              ),
                            ),
                            border:
                                const OutlineInputBorder(),
                          ),
                          validator:
                              (value) {
                            if (value ==
                                    null ||
                                value
                                    .trim()
                                    .isEmpty) {
                              return "Password saat ini wajib diisi";
                            }
                            return null;
                          },
                        ),

                        const SizedBox(
                          height: 14,
                        ),

                        TextFormField(
                          controller:
                              _newPasswordController,
                          obscureText:
                              _obscureNew,
                          decoration:
                              InputDecoration(
                            labelText:
                                "Password Baru",
                            prefixIcon:
                                const Icon(
                              Icons
                                  .lock_reset,
                            ),
                            suffixIcon:
                                IconButton(
                              onPressed:
                                  () {
                                setState(
                                  () {
                                    _obscureNew =
                                        !_obscureNew;
                                  },
                                );
                              },
                              icon: Icon(
                                _obscureNew
                                    ? Icons
                                        .visibility_off
                                    : Icons
                                        .visibility,
                              ),
                            ),
                            border:
                                const OutlineInputBorder(),
                          ),
                          validator:
                              (value) {
                            if (value ==
                                    null ||
                                value
                                    .trim()
                                    .isEmpty) {
                              return "Password baru wajib diisi";
                            }

                            if (value.length <
                                8) {
                              return "Password minimal 8 karakter";
                            }

                            return null;
                          },
                        ),

                        const SizedBox(
                          height: 14,
                        ),

                        TextFormField(
                          controller:
                              _confirmPasswordController,
                          obscureText:
                              _obscureConfirm,
                          decoration:
                              InputDecoration(
                            labelText:
                                "Konfirmasi Password Baru",
                            prefixIcon:
                                const Icon(
                              Icons
                                  .verified_user_outlined,
                            ),
                            suffixIcon:
                                IconButton(
                              onPressed:
                                  () {
                                setState(
                                  () {
                                    _obscureConfirm =
                                        !_obscureConfirm;
                                  },
                                );
                              },
                              icon: Icon(
                                _obscureConfirm
                                    ? Icons
                                        .visibility_off
                                    : Icons
                                        .visibility,
                              ),
                            ),
                            border:
                                const OutlineInputBorder(),
                          ),
                          validator:
                              (value) {
                            if (value ==
                                    null ||
                                value
                                    .trim()
                                    .isEmpty) {
                              return "Konfirmasi password wajib diisi";
                            }

                            if (value !=
                                _newPasswordController
                                    .text) {
                              return "Konfirmasi password tidak sesuai";
                            }

                            return null;
                          },
                        ),

                        if (provider
                                .errorMessage !=
                            null) ...[
                          const SizedBox(
                            height: 16,
                          ),
                          _ErrorBanner(
                            message: provider
                                .errorMessage!,
                          ),
                        ],

                        const SizedBox(
                          height: 24,
                        ),

                        FilledButton.icon(
                          onPressed:
                              provider.isLoading
                                  ? null
                                  : _changePassword,
                          icon:
                              provider.isLoading
                                  ? const SizedBox(
                                      width:
                                          18,
                                      height:
                                          18,
                                      child:
                                          CircularProgressIndicator(
                                        strokeWidth:
                                            2,
                                      ),
                                    )
                                  : const Icon(
                                      Icons
                                          .save_outlined,
                                    ),
                          label: const Text(
                            "Simpan Password Baru",
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

  const _ErrorBanner({
    required this.message,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding:
          const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: Colors.red.shade50,
        borderRadius:
            BorderRadius.circular(12),
        border: Border.all(
          color: Colors.red.shade100,
        ),
      ),
      child: Row(
        children: [
          Icon(
            Icons.error_outline,
            color: Colors.red.shade700,
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Text(
              message,
              style: TextStyle(
                color:
                    Colors.red.shade700,
              ),
            ),
          ),
        ],
      ),
    );
  }
}