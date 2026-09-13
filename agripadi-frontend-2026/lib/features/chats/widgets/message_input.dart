import 'package:agripadi/core/constants/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:image_picker/image_picker.dart';

class MessageInput extends StatefulWidget {
  final bool isLoading;
  final Future<bool> Function(String message) onSendMessage;
  final Future<bool> Function(XFile image) onSendImage;

  const MessageInput({
    super.key,
    required this.isLoading,
    required this.onSendMessage,
    required this.onSendImage,
  });

  @override
  State<MessageInput> createState() => _MessageInputState();
}

class _MessageInputState extends State<MessageInput> {
  final TextEditingController _controller = TextEditingController();
  final FocusNode _focusNode = FocusNode();
  final ImagePicker _picker = ImagePicker();

  bool _isComposing = false;

  @override
  void initState() {
    super.initState();
    _focusNode.addListener(() {
      setState(() {});
    });
  }

  @override
  void dispose() {
    _controller.dispose();
    _focusNode.dispose();
    super.dispose();
  }

  Future<void> _handleSend() async {
    final text = _controller.text.trim();

    if (text.isEmpty) {
      return;
    }

    HapticFeedback.lightImpact();
    _controller.clear();

    setState(() {
      _isComposing = false;
    });

    final sent = await widget.onSendMessage(text);
    if (!sent && mounted) {
      // Pulihkan teks agar pengguna tidak perlu mengetik ulang ketika request
      // gagal karena jaringan atau server sementara.
      _controller.text = text;
      _controller.selection = TextSelection.collapsed(offset: text.length);
      setState(() {
        _isComposing = true;
      });
      _focusNode.requestFocus();
    }
  }

  Future<void> _showImageSourceSheet() async {
    HapticFeedback.selectionClick();

    final source = await showModalBottomSheet<ImageSource>(
      context: context,
      showDragHandle: true,
      isScrollControlled: true,
      useSafeArea: true,
      backgroundColor: Colors.white,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(28)),
      ),
      builder: (context) {
        final bottomInset = MediaQuery.viewInsetsOf(context).bottom;

        return SafeArea(
          child: SingleChildScrollView(
            padding: EdgeInsets.fromLTRB(18, 4, 18, 20 + bottomInset),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text(
                  'Kirim gambar untuk dianalisis',
                  style: TextStyle(
                    fontSize: 18,
                    fontWeight: FontWeight.w900,
                    color: AppColors.textBlack,
                  ),
                ),
                const SizedBox(height: 6),
                const Text(
                  'Pilih sumber gambar tanaman padi yang ingin dianalisis.',
                  style: TextStyle(
                    color: AppColors.textMuted,
                    fontSize: 13,
                    height: 1.45,
                  ),
                ),
                const SizedBox(height: 16),
                _ImageSourceTile(
                  icon: Icons.photo_camera_outlined,
                  title: 'Scan langsung dari kamera',
                  subtitle: 'Cocok untuk fitur scan hama saat demo aplikasi',
                  onTap: () => Navigator.pop(context, ImageSource.camera),
                ),
                const SizedBox(height: 8),
                _ImageSourceTile(
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

    await _pickImage(source);
  }

  Future<void> _pickImage(ImageSource source) async {
    try {
      final picked = await _picker.pickImage(source: source, imageQuality: 80);

      if (picked == null) {
        return;
      }

      await widget.onSendImage(picked);
    } on PlatformException catch (error) {
      if (!mounted) {
        return;
      }

      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            'Gagal membuka ${source == ImageSource.camera ? "kamera" : "galeri"}: ${error.message ?? error.code}',
          ),
        ),
      );
    } catch (error) {
      if (!mounted) {
        return;
      }

      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Gagal memilih gambar: $error')),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      top: false,
      child: Container(
        padding: const EdgeInsets.fromLTRB(14, 10, 14, 14),
        decoration: BoxDecoration(
          color: Colors.white.withOpacity(0.96),
          border: const Border(top: BorderSide(color: AppColors.stroke)),
          boxShadow: [
            BoxShadow(
              color: Colors.black.withOpacity(0.05),
              blurRadius: 18,
              offset: const Offset(0, -8),
            ),
          ],
        ),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.end,
          children: [
            Expanded(
              child: AnimatedContainer(
                duration: const Duration(milliseconds: 180),
                decoration: BoxDecoration(
                  color: AppColors.surfaceSoft,
                  borderRadius: BorderRadius.circular(26),
                  border: Border.all(
                    color: _focusNode.hasFocus
                        ? AppColors.primary.withOpacity(0.48)
                        : AppColors.stroke,
                    width: 1.4,
                  ),
                ),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.end,
                  children: [
                    Padding(
                      padding: const EdgeInsets.only(left: 4, bottom: 3),
                      child: IconButton(
                        tooltip: 'Tambah gambar / scan hama',
                        onPressed: widget.isLoading ? null : _showImageSourceSheet,
                        icon: Icon(
                          Icons.add_photo_alternate_rounded,
                          color: widget.isLoading
                              ? Colors.grey.shade400
                              : AppColors.primary,
                          size: 25,
                        ),
                      ),
                    ),
                    Expanded(
                      child: TextField(
                        controller: _controller,
                        focusNode: _focusNode,
                        enabled: !widget.isLoading,
                        minLines: 1,
                        maxLines: 5,
                        textInputAction: TextInputAction.newline,
                        style: const TextStyle(
                          fontSize: 15,
                          color: AppColors.textBlack,
                          height: 1.35,
                        ),
                        onChanged: (value) {
                          setState(() {
                            _isComposing = value.trim().isNotEmpty;
                          });
                        },
                        decoration: const InputDecoration(
                          hintText: 'Tulis pertanyaan...',
                          hintStyle: TextStyle(color: AppColors.textMuted),
                          border: InputBorder.none,
                          filled: false,
                          contentPadding: EdgeInsets.only(
                            top: 15,
                            bottom: 15,
                            right: 14,
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ),
            const SizedBox(width: 10),
            AnimatedContainer(
              duration: const Duration(milliseconds: 180),
              width: 50,
              height: 50,
              decoration: BoxDecoration(
                gradient: _isComposing && !widget.isLoading
                    ? AppColors.primaryGradient
                    : null,
                color: _isComposing && !widget.isLoading
                    ? null
                    : const Color(0xFFE7ECE9),
                shape: BoxShape.circle,
                boxShadow: _isComposing && !widget.isLoading
                    ? [
                        BoxShadow(
                          color: AppColors.primary.withOpacity(0.24),
                          blurRadius: 14,
                          offset: const Offset(0, 8),
                        ),
                      ]
                    : null,
              ),
              child: IconButton(
                onPressed: (_isComposing && !widget.isLoading) ? _handleSend : null,
                icon: widget.isLoading
                    ? const SizedBox(
                        width: 20,
                        height: 20,
                        child: CircularProgressIndicator(
                          strokeWidth: 2.5,
                          color: AppColors.primary,
                        ),
                      )
                    : Icon(
                        Icons.send_rounded,
                        size: 22,
                        color: _isComposing
                            ? Colors.white
                            : const Color(0xFFAEB7B1),
                      ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _ImageSourceTile extends StatelessWidget {
  final IconData icon;
  final String title;
  final String subtitle;
  final VoidCallback onTap;

  const _ImageSourceTile({
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
