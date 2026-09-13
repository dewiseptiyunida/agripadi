import 'dart:ui';

import 'package:agripadi/features/auth/providers/auth_provider.dart';
import 'package:agripadi/features/auth/screens/profile_screen.dart';
import 'package:agripadi/features/chats/providers/chat_provider.dart';
import 'package:agripadi/features/conversation/models/conversation_item.dart';
import 'package:agripadi/features/conversation/providers/conversation_provider.dart';
import 'package:agripadi/features/conversation/widgets/conversation_tile.dart';
import 'package:agripadi/features/conversation/widgets/empty_conversation.dart';

import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

class ConversationSidebar extends StatefulWidget {
  const ConversationSidebar({super.key});

  @override
  State<ConversationSidebar> createState() => _ConversationSidebarState();
}

class _ConversationSidebarState extends State<ConversationSidebar> {
  bool _initialized = false;

  final Set<String> _selectedIds = {};

  bool get _isSelectionMode => _selectedIds.isNotEmpty;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();

    if (!_initialized) {
      _initialized = true;

      WidgetsBinding.instance.addPostFrameCallback((_) {
        context.read<ConversationProvider>().getUserConversations();
      });
    }
  }

  // =========================================================
  // SELECTION
  // =========================================================

  void _toggleSelection(String id) {
    setState(() {
      if (_selectedIds.contains(id)) {
        _selectedIds.remove(id);
      } else {
        _selectedIds.add(id);
      }
    });
  }

  void _clearSelection() {
    setState(() {
      _selectedIds.clear();
    });
  }

  // =========================================================
  // NEW CHAT
  // =========================================================

  void _startNewChat(bool isMobile) {
    final chatProvider = context.read<ChatProvider>();
    if (chatProvider.isBusy) {
      return;
    }

    if (_isSelectionMode) {
      _clearSelection();
    }

    chatProvider.clearChat();
    context.read<ConversationProvider>().startNewConversation();

    if (isMobile && mounted) {
      Navigator.of(context).pop();
    }
  }

  // =========================================================
  // MODERN CONFIRM DIALOG
  // =========================================================

  Future<bool?> _showModernConfirmDialog({
    required BuildContext context,
    required String title,
    required String content,
  }) {
    return showDialog<bool>(
      context: context,

      builder: (_) {
        return AlertDialog(
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(20),
          ),

          title: Row(
            children: [
              Icon(Icons.warning_amber_rounded, color: Colors.red.shade400),

              const SizedBox(width: 10),

              Expanded(
                child: Text(
                  title,

                  maxLines: 1,

                  overflow: TextOverflow.ellipsis,

                  style: const TextStyle(fontWeight: FontWeight.bold),
                ),
              ),
            ],
          ),

          content: Text(content, style: TextStyle(color: Colors.grey.shade700)),

          actionsPadding: const EdgeInsets.symmetric(
            horizontal: 16,
            vertical: 12,
          ),

          actions: [
            TextButton(
              onPressed: () {
                Navigator.pop(context, false);
              },

              child: const Text("Batal"),
            ),

            ElevatedButton(
              style: ElevatedButton.styleFrom(
                backgroundColor: Colors.red.shade50,

                foregroundColor: Colors.red.shade600,

                elevation: 0,

                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(10),
                ),
              ),

              onPressed: () {
                Navigator.pop(context, true);
              },

              child: const Text("Hapus"),
            ),
          ],
        );
      },
    );
  }

  // =========================================================
  // DELETE SINGLE
  // =========================================================

  Future<void> _deleteConversation(
    BuildContext context,
    ConversationItem item,
  ) async {
    final provider = context.read<ConversationProvider>();

    final chatProvider = context.read<ChatProvider>();

    final confirm = await _showModernConfirmDialog(
      context: context,

      title: "Hapus",

      content: "Yakin ingin menghapus \"${item.title}\"?",
    );

    if (confirm != true) {
      return;
    }

    final deleted = await provider.deleteConversation(item.id);
    if (!deleted) {
      return;
    }

    if (chatProvider.conversationId == item.id) {
      chatProvider.clearChat();
    }
  }

  // =========================================================
  // DELETE MULTIPLE
  // =========================================================

  Future<void> _deleteSelected(BuildContext context) async {
    final provider = context.read<ConversationProvider>();

    final chatProvider = context.read<ChatProvider>();

    final ids = _selectedIds.toList();

    final shouldDeleteAll =
        provider.conversations.isNotEmpty &&
        ids.length == provider.conversations.length;

    final confirm = await _showModernConfirmDialog(
      context: context,

      title: shouldDeleteAll
          ? "Hapus Semua Riwayat"
          : "Hapus ${ids.length} Percakapan",

      content: shouldDeleteAll
          ? "Semua riwayat percakapan pada akun ini akan dihapus dari backend dan aplikasi. Tindakan ini tidak dapat dibatalkan. Lanjutkan?"
          : "Percakapan yang dipilih akan dihapus. Tindakan ini tidak dapat dibatalkan. Lanjutkan?",
    );

    if (confirm != true) {
      return;
    }

    final deleted = shouldDeleteAll
        ? await provider.deleteAllConversations()
        : await provider.deleteManyConversations(ids);

    if (!deleted) {
      return;
    }

    if (shouldDeleteAll || ids.contains(chatProvider.conversationId)) {
      chatProvider.clearChat();
    }

    _clearSelection();
  }

  // =========================================================
  // HEADER
  // =========================================================

  Widget _buildHeader(ConversationProvider provider, bool isMobile) {
    return Container(
      padding: const EdgeInsets.fromLTRB(16, 20, 12, 16),

      decoration: BoxDecoration(
        color: Colors.white,

        border: Border(bottom: BorderSide(color: Colors.grey.shade200)),
      ),

      child: AnimatedSwitcher(
        duration: const Duration(milliseconds: 300),

        child: _isSelectionMode
            ? _buildSelectionHeader(provider)
            : _buildNormalHeader(provider, isMobile),
      ),
    );
  }

  Widget _buildNormalHeader(ConversationProvider provider, bool isMobile) {
    return Column(
      key: const ValueKey('normal_header'),
      mainAxisSize: MainAxisSize.min,
      children: [
        Row(
          children: [
            Container(
              width: 40,
              height: 40,
              padding: const EdgeInsets.all(8),
              decoration: BoxDecoration(
                gradient: LinearGradient(
                  colors: [Colors.green.shade50, Colors.teal.shade50],
                ),
                borderRadius: BorderRadius.circular(12),
                border: Border.all(color: Colors.green.shade100),
              ),
              child: Image.asset("assets/icon/app_icon.png"),
            ),
            const SizedBox(width: 12),
            const Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    "AgriPadi",
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                      fontSize: 18,
                      fontWeight: FontWeight.w800,
                      letterSpacing: -0.5,
                    ),
                  ),
                  SizedBox(height: 2),
                  Text(
                    "Riwayat diagnosis",
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                      fontSize: 12,
                      color: Color(0xFF6B7280),
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ],
              ),
            ),
            if (isMobile)
              IconButton(
                tooltip: "Tutup sidebar",
                padding: const EdgeInsets.all(8),
                constraints: const BoxConstraints(),
                onPressed: () => Navigator.of(context).pop(),
                icon: Icon(Icons.close_rounded, color: Colors.grey.shade700),
              ),
          ],
        ),
        const SizedBox(height: 14),
        SizedBox(
          width: double.infinity,
          height: 46,
          child: FilledButton.icon(
            style: FilledButton.styleFrom(
              backgroundColor: Colors.green.shade700,
              foregroundColor: Colors.white,
              elevation: 0,
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(14),
              ),
            ),
            onPressed: () => _startNewChat(isMobile),
            icon: const Icon(Icons.edit_square, size: 18),
            label: const Text(
              "Pesan Baru",
              style: TextStyle(fontWeight: FontWeight.w800),
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildSelectionHeader(ConversationProvider provider) {
    return Row(
      key: const ValueKey('selection_header'),

      children: [
        IconButton(
          padding: const EdgeInsets.all(4),

          constraints: const BoxConstraints(),

          onPressed: _clearSelection,

          icon: Icon(Icons.close, color: Colors.grey.shade700),
        ),

        const SizedBox(width: 8),

        Expanded(
          child: Text(
            "${_selectedIds.length} dipilih",

            maxLines: 1,

            overflow: TextOverflow.ellipsis,

            style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
          ),
        ),

        IconButton(
          padding: const EdgeInsets.all(4),

          constraints: const BoxConstraints(),

          tooltip: "Pilih Semua",

          onPressed: () {
            setState(() {
              _selectedIds.addAll(provider.conversations.map((e) => e.id));
            });
          },

          icon: Icon(Icons.select_all, color: Colors.grey.shade600),
        ),

        const SizedBox(width: 8),

        IconButton(
          padding: const EdgeInsets.all(4),

          constraints: const BoxConstraints(),

          tooltip: "Hapus Terpilih",

          onPressed: () => _deleteSelected(context),

          icon: const Icon(Icons.delete_outline, color: Colors.red),
        ),
      ],
    );
  }

  // =========================================================
  // PROFILE FOOTER
  // =========================================================

  Widget _buildProfileFooter() {
    return SafeArea(
      top: false,
      child: Container(
        padding: const EdgeInsets.fromLTRB(14, 10, 14, 14),
        decoration: BoxDecoration(
          color: Colors.white,
          border: Border(top: BorderSide(color: Colors.grey.shade200)),
        ),
        child: Consumer<AuthProvider>(
          builder: (_, authProvider, _) {
            final user = authProvider.user;
            final title = user?.name.isNotEmpty == true ? user!.name : "Profil";
            final subtitle = user?.phoneNumber.isNotEmpty == true
                ? user!.phoneNumber
                : "Kelola akun";

            return Material(
              color: Colors.transparent,
              child: InkWell(
                borderRadius: BorderRadius.circular(14),
                onTap: () {
                  Navigator.push(
                    context,
                    MaterialPageRoute(builder: (_) => const ProfileScreen()),
                  );
                },
                child: Ink(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 14,
                    vertical: 12,
                  ),
                  decoration: BoxDecoration(
                    color: Colors.green.shade50,
                    borderRadius: BorderRadius.circular(14),
                    border: Border.all(color: Colors.green.shade100),
                  ),
                  child: Row(
                    children: [
                      CircleAvatar(
                        radius: 18,
                        backgroundColor: Colors.green.shade100,
                        child: Icon(
                          Icons.person_outline,
                          color: Colors.green.shade800,
                        ),
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              title,
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: const TextStyle(
                                fontSize: 14,
                                fontWeight: FontWeight.w700,
                              ),
                            ),
                            Text(
                              subtitle,
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: TextStyle(
                                fontSize: 12,
                                color: Colors.grey.shade700,
                              ),
                            ),
                          ],
                        ),
                      ),
                      Icon(Icons.chevron_right, color: Colors.green.shade700),
                    ],
                  ),
                ),
              ),
            );
          },
        ),
      ),
    );
  }

  // =========================================================
  // CONVERSATION LIST
  // =========================================================

  Widget _buildConversationList(ConversationProvider provider, bool isMobile) {
    return RefreshIndicator(
      onRefresh: provider.refresh,

      color: Colors.green,

      backgroundColor: Colors.white,

      child: ListView.builder(
        physics: const AlwaysScrollableScrollPhysics(
          parent: BouncingScrollPhysics(),
        ),

        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 16),

        itemCount: provider.conversations.length,

        itemBuilder: (_, index) {
          final item = provider.conversations[index];

          final isSelected = _selectedIds.contains(item.id);

          return Padding(
            padding: const EdgeInsets.only(bottom: 8),

            child: GestureDetector(
              onLongPress: () => _toggleSelection(item.id),

              child: AnimatedContainer(
                duration: const Duration(milliseconds: 200),

                decoration: BoxDecoration(
                  borderRadius: BorderRadius.circular(12),

                  color: isSelected ? Colors.green.shade50 : Colors.transparent,

                  border: isSelected
                      ? Border.all(color: Colors.green.shade200)
                      : Border.all(color: Colors.transparent),
                ),

                child: ConversationTile(
                  conversation: item,

                  isSelected: _isSelectionMode
                      ? isSelected
                      : provider.selectedConversation?.id == item.id,

                  onTap: () async {
                    final chatProvider = context.read<ChatProvider>();
                    if (chatProvider.isBusy) {
                      return;
                    }

                    if (_isSelectionMode) {
                      _toggleSelection(item.id);

                      return;
                    }

                    final loaded = await chatProvider.loadConversation(item.id);

                    if (!loaded) {
                      return;
                    }

                    provider.selectConversation(item);

                    if (isMobile && mounted) {
                      Navigator.pop(context);
                    }
                  },

                  onDelete: () => _deleteConversation(context, item),
                ),
              ),
            ),
          );
        },
      ),
    );
  }

  // =========================================================
  // ERROR STATE
  // =========================================================

  Widget _buildErrorState(ConversationProvider provider) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),

        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,

          children: [
            Container(
              padding: const EdgeInsets.all(16),

              decoration: BoxDecoration(
                color: Colors.red.shade50,

                shape: BoxShape.circle,
              ),

              child: Icon(
                Icons.cloud_off_rounded,

                size: 48,

                color: Colors.red.shade300,
              ),
            ),

            const SizedBox(height: 24),

            Text(
              provider.errorMessage!,

              textAlign: TextAlign.center,

              style: TextStyle(fontSize: 16, color: Colors.grey.shade700),
            ),

            const SizedBox(height: 24),

            FilledButton.icon(
              style: FilledButton.styleFrom(
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(12),
                ),
              ),

              onPressed: provider.getUserConversations,

              icon: const Icon(Icons.refresh),

              label: const Text("Coba Lagi"),
            ),
          ],
        ),
      ),
    );
  }

  // =========================================================
  // BUILD
  // =========================================================

  @override
  Widget build(BuildContext context) {
    final isMobile = MediaQuery.of(context).size.width < 768;

    return Container(
      width: isMobile ? null : 320,

      decoration: BoxDecoration(
        color: const Color(0xFFFAFAFA),

        border: Border(right: BorderSide(color: Colors.grey.shade200)),
      ),

      child: Consumer<ConversationProvider>(
        builder: (_, provider, _) {
          // ===================================
          // CONTENT
          // ===================================

          return Stack(
            children: [
              Column(
                children: [
                  _buildHeader(provider, isMobile),

                  Expanded(child: _buildSidebarContent(provider, isMobile)),

                  _buildProfileFooter(),
                ],
              ),

              // ===============================
              // DELETE OVERLAY
              // ===============================
              if (provider.isDeleting)
                Positioned.fill(
                  child: BackdropFilter(
                    filter: ImageFilter.blur(sigmaX: 3, sigmaY: 3),

                    child: Container(
                      color: Colors.white.withOpacity(0.5),

                      child: Center(
                        child: Container(
                          padding: const EdgeInsets.all(20),

                          decoration: BoxDecoration(
                            color: Colors.white,

                            borderRadius: BorderRadius.circular(16),

                            boxShadow: [
                              BoxShadow(
                                color: Colors.black.withOpacity(0.1),

                                blurRadius: 10,
                              ),
                            ],
                          ),

                          child: const CircularProgressIndicator(),
                        ),
                      ),
                    ),
                  ),
                ),
            ],
          );
        },
      ),
    );
  }

  Widget _buildSidebarContent(ConversationProvider provider, bool isMobile) {
    if (provider.isLoading && provider.conversations.isEmpty) {
      return const Center(child: CircularProgressIndicator());
    }

    if (provider.errorMessage != null && provider.conversations.isEmpty) {
      return _buildErrorState(provider);
    }

    if (provider.conversations.isEmpty) {
      return const EmptyConversation();
    }

    return _buildConversationList(provider, isMobile);
  }
}
