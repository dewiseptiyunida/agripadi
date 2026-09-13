import 'package:agripadi/core/constants/app_colors.dart';
import 'package:agripadi/features/auth/providers/auth_provider.dart';
import 'package:agripadi/features/chats/models/chat_action.dart';
import 'package:agripadi/features/chats/models/chat_message.dart';
import 'package:agripadi/features/chats/providers/chat_provider.dart';
import 'package:agripadi/features/chats/widgets/chat_bubble.dart';
import 'package:agripadi/features/chats/widgets/empty_chat.dart';
import 'package:agripadi/features/chats/widgets/message_input.dart';
import 'package:agripadi/features/chats/widgets/typing_indicator.dart';
import 'package:agripadi/features/conversation/providers/conversation_provider.dart';
import 'package:agripadi/features/conversation/screens/conversation_layout.dart';
import 'package:agripadi/features/onboarding/widgets/onboarding_dialog.dart';
import 'package:flutter/material.dart';
import 'package:image_picker/image_picker.dart';
import 'package:provider/provider.dart';
import 'package:shared_preferences/shared_preferences.dart';

class ChatScreen extends StatefulWidget {
  const ChatScreen({super.key});

  @override
  State<ChatScreen> createState() => _ChatScreenState();
}

class _ChatScreenState extends State<ChatScreen> {
  final ScrollController _scrollController = ScrollController();
  final ImagePicker _picker = ImagePicker();

  @override
  void initState() {
    super.initState();

    WidgetsBinding.instance.addPostFrameCallback((_) async {
      context.read<ChatProvider>().initialize();

      final authProvider = context.read<AuthProvider>();

      if (authProvider.user == null) {
        await authProvider.loadProfile();
      }

      if (!mounted) {
        return;
      }

      await _showOnboardingIfNeeded();
    });
  }

  @override
  void dispose() {
    _scrollController.dispose();
    super.dispose();
  }

  void _scrollToBottom() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!_scrollController.hasClients) {
        return;
      }

      _scrollController.animateTo(
        _scrollController.position.maxScrollExtent,
        duration: const Duration(milliseconds: 300),
        curve: Curves.easeOut,
      );
    });
  }

  Future<void> _showOnboardingIfNeeded() async {
    final prefs = await SharedPreferences.getInstance();
    final completed = prefs.getBool('agri_padi_onboarding_completed') ?? false;

    if (completed || !mounted) {
      return;
    }

    await showDialog(
      context: context,
      barrierDismissible: false,
      builder: (_) => OnboardingDialog(
        onFinished: () async {
          await prefs.setBool('agri_padi_onboarding_completed', true);
        },
      ),
    );
  }

  Future<bool> _sendMessage(String message) async {
    final provider = context.read<ChatProvider>();
    final sent = await provider.sendMessage(message: message);

    if (sent && mounted) {
      await _syncConversationSidebar(provider.conversationId);
    }

    if (mounted) {
      _scrollToBottom();
    }

    return sent;
  }

  Future<bool> _sendImage(XFile image) async {
    final provider = context.read<ChatProvider>();
    final sent = await provider.sendImageMessage(imageFile: image);

    if (sent && mounted) {
      await _syncConversationSidebar(provider.conversationId);
    }

    if (mounted) {
      _scrollToBottom();
    }

    return sent;
  }

  Future<void> _syncConversationSidebar(String? conversationId) async {
    final conversationProvider = context.read<ConversationProvider>();
    await conversationProvider.refresh();
    conversationProvider.selectConversationById(conversationId);
  }

  Future<void> _handleActionSelected(ChatAction action) async {
    if (context.read<ChatProvider>().isBusy) {
      return;
    }

    if (action.isUploadImage) {
      await _showImageSourceSheet();
      return;
    }

    if (action.isContinueConsultation) {
      if (!mounted) {
        return;
      }

      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text(
            'Mode konsultasi aktif. Silakan tulis pertanyaan lanjutan di kolom chat.',
          ),
        ),
      );
      return;
    }

    final value = action.value.trim();
    if (value.isEmpty) {
      return;
    }

    await _sendMessage(value);
  }

  bool _isLatestActionMessage(List<ChatMessage> messages, int index) {
    for (var i = messages.length - 1; i >= 0; i--) {
      final message = messages[i];
      if (!message.isUser && message.hasActions) {
        return i == index;
      }
    }

    return false;
  }

  Future<void> _showImageSourceSheet() async {
    final source = await showModalBottomSheet<ImageSource>(
      context: context,
      showDragHandle: true,
      backgroundColor: Colors.white,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(28)),
      ),
      builder: (context) {
        return SafeArea(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(18, 4, 18, 20),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text(
                  'Kirim gambar untuk diagnosis baru',
                  style: TextStyle(
                    fontSize: 18,
                    fontWeight: FontWeight.w900,
                    color: AppColors.textBlack,
                  ),
                ),
                const SizedBox(height: 6),
                const Text(
                  'Pilih kamera atau galeri untuk memulai ulang flow diagnosis hama.',
                  style: TextStyle(
                    color: AppColors.textMuted,
                    fontSize: 13,
                    height: 1.45,
                  ),
                ),
                const SizedBox(height: 16),
                _ActionImageSourceTile(
                  icon: Icons.photo_camera_outlined,
                  title: 'Scan langsung dari kamera',
                  subtitle: 'Ambil foto hama atau gejala tanaman secara langsung',
                  onTap: () => Navigator.pop(context, ImageSource.camera),
                ),
                const SizedBox(height: 8),
                _ActionImageSourceTile(
                  icon: Icons.photo_library_outlined,
                  title: 'Pilih gambar dari galeri',
                  subtitle: 'Gunakan foto tanaman padi yang sudah tersedia',
                  onTap: () => Navigator.pop(context, ImageSource.gallery),
                ),
              ],
            ),
          ),
        );
      },
    );

    if (source == null) {
      return;
    }

    try {
      final picked = await _picker.pickImage(source: source, imageQuality: 80);
      if (picked == null) {
        return;
      }

      await _sendImage(picked);
    } catch (error) {
      if (!mounted) {
        return;
      }
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Gagal membuka gambar: $error')),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    return Consumer<ChatProvider>(
      builder: (_, provider, _) {
        WidgetsBinding.instance.addPostFrameCallback((_) {
          _scrollToBottom();
        });

        return ConversationLayout(
          child: GestureDetector(
            onTap: () => FocusScope.of(context).unfocus(),
            child: SafeArea(
              top: false,
              child: Column(
                children: [
                  _ChatStatusStrip(
                    messageCount: provider.messages.length,
                    isLoading: provider.isLoading || provider.isTyping,
                  ),
                  Expanded(
                    child: provider.messages.isEmpty
                        ? EmptyChat(onPromptSelected: _sendMessage)
                        : ListView.builder(
                            controller: _scrollController,
                            physics: const BouncingScrollPhysics(),
                            padding: const EdgeInsets.fromLTRB(0, 16, 0, 18),
                            itemCount: provider.messages.length +
                                (provider.isTyping ? 1 : 0),
                            itemBuilder: (_, index) {
                              if (provider.isTyping &&
                                  index == provider.messages.length) {
                                return const TypingIndicator();
                              }

                              final message = provider.messages[index];
                              return ChatBubble(
                                message: message,
                                showActions: _isLatestActionMessage(
                                  provider.messages,
                                  index,
                                ),
                                onActionSelected: _handleActionSelected,
                              );
                            },
                          ),
                  ),
                  if (provider.errorMessage != null)
                    _ErrorBanner(
                      message: provider.errorMessage!,
                      onClose: provider.clearError,
                    ),
                  MessageInput(
                    isLoading: provider.isBusy,
                    onSendMessage: _sendMessage,
                    onSendImage: _sendImage,
                  ),
                ],
              ),
            ),
          ),
        );
      },
    );
  }
}


class _ActionImageSourceTile extends StatelessWidget {
  final IconData icon;
  final String title;
  final String subtitle;
  final VoidCallback onTap;

  const _ActionImageSourceTile({
    required this.icon,
    required this.title,
    required this.subtitle,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Material(
      color: AppColors.surfaceSoft,
      borderRadius: BorderRadius.circular(18),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(18),
        child: Container(
          padding: const EdgeInsets.all(14),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(18),
            border: Border.all(color: AppColors.stroke),
          ),
          child: Row(
            children: [
              Container(
                width: 46,
                height: 46,
                decoration: BoxDecoration(
                  color: AppColors.primaryLight,
                  borderRadius: BorderRadius.circular(16),
                ),
                child: Icon(icon, color: AppColors.primary),
              ),
              const SizedBox(width: 14),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      title,
                      style: const TextStyle(
                        fontSize: 15,
                        fontWeight: FontWeight.w800,
                        color: AppColors.textBlack,
                      ),
                    ),
                    const SizedBox(height: 3),
                    Text(
                      subtitle,
                      style: const TextStyle(
                        fontSize: 13,
                        height: 1.35,
                        color: AppColors.textMuted,
                      ),
                    ),
                  ],
                ),
              ),
              const Icon(Icons.chevron_right_rounded, color: AppColors.textMuted),
            ],
          ),
        ),
      ),
    );
  }
}

class _ChatStatusStrip extends StatelessWidget {
  final int messageCount;
  final bool isLoading;

  const _ChatStatusStrip({
    required this.messageCount,
    required this.isLoading,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      margin: const EdgeInsets.fromLTRB(16, 12, 16, 0),
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(20),
        border: Border.all(color: AppColors.stroke),
        boxShadow: [
          BoxShadow(
            color: Colors.black.withOpacity(0.035),
            blurRadius: 18,
            offset: const Offset(0, 8),
          ),
        ],
      ),
      child: Row(
        children: [
          Container(
            width: 38,
            height: 38,
            decoration: BoxDecoration(
              color: AppColors.primaryLight,
              borderRadius: BorderRadius.circular(14),
            ),
            child: const Icon(
              Icons.smart_toy_outlined,
              color: AppColors.primary,
              size: 20,
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text(
                  'Mode Chat Diagnosis Hama Padi',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    fontWeight: FontWeight.w900,
                    color: AppColors.textBlack,
                    fontSize: 13.5,
                  ),
                ),
                const SizedBox(height: 3),
                Text(
                  messageCount > 0
                      ? 'Konsultasi dan analisis gambar on-device • $messageCount pesan'
                      : 'Konsultasi hama padi berbasis chat dan CNN on-device',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                    color: AppColors.textMuted,
                    fontSize: 12.2,
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(width: 10),
          _ModeChip(isLoading: isLoading),
        ],
      ),
    );
  }
}

class _ModeChip extends StatelessWidget {
  final bool isLoading;

  const _ModeChip({required this.isLoading});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
      decoration: BoxDecoration(
        color: isLoading ? const Color(0xFFFFF8E8) : AppColors.primaryLight,
        borderRadius: BorderRadius.circular(999),
        border: Border.all(
          color: isLoading ? const Color(0xFFF4DDA2) : AppColors.strokeStrong,
        ),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            isLoading ? Icons.sync_rounded : Icons.memory_rounded,
            size: 15,
            color: isLoading ? AppColors.warning : AppColors.primary,
          ),
          const SizedBox(width: 6),
          Text(
            isLoading ? 'Memproses' : 'Siap',
            style: TextStyle(
              color: isLoading ? AppColors.warning : AppColors.primary,
              fontWeight: FontWeight.w900,
              fontSize: 12,
            ),
          ),
        ],
      ),
    );
  }
}

class _ErrorBanner extends StatelessWidget {
  final String message;
  final VoidCallback onClose;

  const _ErrorBanner({required this.message, required this.onClose});

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: Colors.red.shade50,
        borderRadius: BorderRadius.circular(18),
        border: Border.all(color: Colors.red.shade200),
      ),
      child: Row(
        children: [
          Icon(Icons.error_outline, color: Colors.red.shade700),
          const SizedBox(width: 12),
          Expanded(
            child: Text(
              message,
              style: TextStyle(color: Colors.red.shade700, height: 1.4),
            ),
          ),
          IconButton(
            onPressed: onClose,
            icon: const Icon(Icons.close_rounded),
          ),
        ],
      ),
    );
  }
}
