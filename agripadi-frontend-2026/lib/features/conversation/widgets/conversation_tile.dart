import 'package:agripadi/features/conversation/models/conversation_item.dart';
import 'package:flutter/material.dart';
import 'package:intl/intl.dart';

class ConversationTile extends StatelessWidget {
  final ConversationItem conversation;

  final bool isSelected;

  final VoidCallback onTap;

  final VoidCallback? onDelete;

  const ConversationTile({
    super.key,
    required this.conversation,
    required this.isSelected,
    required this.onTap,
    this.onDelete,
  });

  // =========================================
  // FORMAT TIME
  // =========================================

  String _formatTime(DateTime? time) {
    if (time == null) {
      return "";
    }

    final now = DateTime.now();

    final difference = now.difference(time);

    if (difference.inDays == 0) {
      return DateFormat("HH:mm").format(time);
    }

    if (difference.inDays == 1) {
      return "Kemarin";
    }

    if (difference.inDays < 7) {
      return DateFormat("EEE").format(time);
    }

    return DateFormat("dd/MM").format(time);
  }

  // =========================================
  // MESSAGE PREVIEW
  // =========================================

  String _buildPreview() {
    if (conversation.isImageMessage) {
      return "Gambar dikirim";
    }

    return conversation.lastMessage;
  }

  @override
  Widget build(BuildContext context) {
    final preview = _buildPreview();

    final time = _formatTime(conversation.lastMessageAt);

    return AnimatedContainer(
      duration: const Duration(milliseconds: 180),

      margin: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),

      decoration: BoxDecoration(
        color: isSelected ? const Color(0xFF2D6A4F) : Colors.transparent,

        borderRadius: BorderRadius.circular(18),
      ),

      child: Material(
        color: Colors.transparent,

        child: InkWell(
          borderRadius: BorderRadius.circular(18),

          splashColor: Colors.white.withOpacity(0.08),

          highlightColor: Colors.white.withOpacity(0.04),

          hoverColor: const Color(0xFFF5F7F6),

          onTap: onTap,

          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 14),

            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,

              children: [
                // =========================
                // ICON
                // =========================
                Container(
                  width: 46,
                  height: 46,

                  decoration: BoxDecoration(
                    color: isSelected
                        ? Colors.white.withOpacity(0.14)
                        : const Color(0xFFF1F5F3),

                    borderRadius: BorderRadius.circular(14),
                  ),

                  child: Icon(
                    conversation.isImageMessage
                        ? Icons.image_rounded
                        : Icons.chat_bubble_rounded,

                    color: isSelected ? Colors.white : const Color(0xFF2D6A4F),

                    size: 22,
                  ),
                ),

                const SizedBox(width: 14),

                // =========================
                // CONTENT
                // =========================
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,

                    children: [
                      Row(
                        children: [
                          Expanded(
                            child: Text(
                              conversation.title.trim().isEmpty
                                  ? ""
                                  : conversation.title,

                              maxLines: 1,

                              overflow: TextOverflow.ellipsis,

                              style: TextStyle(
                                color: isSelected
                                    ? Colors.white
                                    : Colors.black87,

                                fontWeight: FontWeight.w700,

                                fontSize: 15,
                              ),
                            ),
                          ),

                          if (time.isNotEmpty)
                            Padding(
                              padding: const EdgeInsets.only(left: 8),

                              child: Text(
                                time,

                                style: TextStyle(
                                  color: isSelected
                                      ? Colors.white70
                                      : Colors.grey.shade500,

                                  fontSize: 11,
                                ),
                              ),
                            ),
                        ],
                      ),

                      const SizedBox(height: 6),

                      Text(
                        preview,

                        maxLines: 1,

                        overflow: TextOverflow.ellipsis,

                        style: TextStyle(
                          color: isSelected
                              ? Colors.white70
                              : Colors.grey.shade600,

                          fontSize: 13,

                          height: 1.35,
                        ),
                      ),
                    ],
                  ),
                ),

                if (onDelete != null)
                  Padding(
                    padding: const EdgeInsets.only(left: 8.0),
                    child: IconButton(
                      onPressed: onDelete,
                      icon: const Icon(Icons.delete_outline),
                      iconSize: 20,
                      padding: EdgeInsets.zero,
                      constraints: const BoxConstraints(),
                      color: isSelected ? Colors.white70 : Colors.grey.shade400,
                    ),
                  ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
