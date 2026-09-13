import 'package:agripadi/core/constants/app_colors.dart';
import 'package:agripadi/features/chats/models/chat_action.dart';
import 'package:agripadi/features/chats/models/chat_message.dart';
import 'package:agripadi/features/chats/widgets/image_attachment.dart';
import 'package:flutter/material.dart';

class ChatBubble extends StatelessWidget {
  final ChatMessage message;
  final ValueChanged<ChatAction>? onActionSelected;
  final bool showActions;

  const ChatBubble({
    super.key,
    required this.message,
    this.onActionSelected,
    this.showActions = true,
  });

  @override
  Widget build(BuildContext context) {
    final isUser = message.isUser;

    return Align(
      alignment: isUser ? Alignment.centerRight : Alignment.centerLeft,
      child: Container(
        constraints: const BoxConstraints(maxWidth: 760),
        margin: EdgeInsets.symmetric(
          horizontal: message.hasRecommendation ? 8 : 16,
          vertical: 8,
        ),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisAlignment:
              isUser ? MainAxisAlignment.end : MainAxisAlignment.start,
          children: [
            if (!isUser) ...[
              const _AssistantAvatar(),
              const SizedBox(width: 10),
            ],
            Flexible(
              child: Container(
                padding: EdgeInsets.all(message.hasRecommendation ? 10 : 16),
                decoration: BoxDecoration(
                  color: isUser ? AppColors.userBubble : AppColors.aiBubble,
                  borderRadius: BorderRadius.only(
                    topLeft: const Radius.circular(22),
                    topRight: const Radius.circular(22),
                    bottomLeft: Radius.circular(isUser ? 22 : 8),
                    bottomRight: Radius.circular(isUser ? 8 : 22),
                  ),
                  border: isUser
                      ? null
                      : Border.all(color: AppColors.stroke),
                  boxShadow: [
                    BoxShadow(
                      color: Colors.black.withOpacity(0.04),
                      blurRadius: 16,
                      offset: const Offset(0, 8),
                    ),
                  ],
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    if (message.hasAttachment) ...[
                      ...message.attachments.map(
                        (attachment) => Padding(
                          padding: const EdgeInsets.only(bottom: 14),
                          child: ImageAttachment(attachment: attachment),
                        ),
                      ),
                    ],
                    if (message.message.trim().isNotEmpty &&
                        !message.hasRecommendation)
                      SelectableText(
                        message.message.trim(),
                        style: TextStyle(
                          fontSize: 15.5,
                          height: 1.65,
                          color: isUser ? AppColors.textWhite : AppColors.textBlack,
                          fontWeight: FontWeight.w500,
                        ),
                      ),
                    if (message.hasRecommendation) ...[
                      _RecommendationCard(message: message),
                      if (message.message.trim().isNotEmpty) ...[
                        const SizedBox(height: 12),
                        _FullDiagnosisDetails(text: message.message.trim()),
                      ],
                    ],
                    if (!isUser && showActions && message.hasActions) ...[
                      SizedBox(height: message.hasRecommendation ? 12 : 14),
                      _ActionButtonSection(
                        actions: message.actions,
                        onSelected: onActionSelected,
                      ),
                    ],
                  ],
                ),
              ),
            ),
            if (isUser) ...[
              const SizedBox(width: 10),
              const _UserAvatar(),
            ],
          ],
        ),
      ),
    );
  }
}

class _AssistantAvatar extends StatelessWidget {
  const _AssistantAvatar();

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 38,
      height: 38,
      decoration: BoxDecoration(
        gradient: AppColors.primaryGradient,
        shape: BoxShape.circle,
      ),
      child: const Icon(Icons.eco_rounded, size: 20, color: Colors.white),
    );
  }
}

class _UserAvatar extends StatelessWidget {
  const _UserAvatar();

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 34,
      height: 34,
      decoration: BoxDecoration(
        color: AppColors.primaryLight,
        shape: BoxShape.circle,
        border: Border.all(color: Colors.white, width: 2),
      ),
      child: const Icon(Icons.person_rounded, size: 18, color: AppColors.primary),
    );
  }
}


class _FullDiagnosisDetails extends StatelessWidget {
  final String text;

  const _FullDiagnosisDetails({required this.text});

  @override
  Widget build(BuildContext context) {
    return Theme(
      data: Theme.of(context).copyWith(dividerColor: Colors.transparent),
      child: Container(
        decoration: BoxDecoration(
          color: Colors.white,
          borderRadius: BorderRadius.circular(14),
          border: Border.all(color: AppColors.stroke),
        ),
        child: ExpansionTile(
          tilePadding: const EdgeInsets.symmetric(horizontal: 12),
          childrenPadding: const EdgeInsets.fromLTRB(12, 0, 12, 12),
          initiallyExpanded: false,
          leading: const Icon(
            Icons.article_outlined,
            size: 18,
            color: AppColors.primary,
          ),
          title: const Text(
            'Detail diagnosis lengkap',
            style: TextStyle(
              fontSize: 13.2,
              fontWeight: FontWeight.w800,
              color: AppColors.textBlack,
            ),
          ),
          subtitle: const Text(
            'Berisi kesimpulan, waktu aplikasi, cara aplikasi, tindak lanjut, dan keamanan.',
            style: TextStyle(
              fontSize: 11.8,
              height: 1.3,
              color: AppColors.textMuted,
            ),
          ),
          children: [
            SelectableText(
              text,
              style: const TextStyle(
                fontSize: 12.8,
                height: 1.5,
                color: AppColors.textBlack,
                fontWeight: FontWeight.w500,
              ),
            ),
          ],
        ),
      ),
    );
  }
}


class _ActionButtonSection extends StatelessWidget {
  final List<ChatAction> actions;
  final ValueChanged<ChatAction>? onSelected;

  const _ActionButtonSection({
    required this.actions,
    required this.onSelected,
  });

  @override
  Widget build(BuildContext context) {
    final visibleActions = actions
        .where((item) => item.label.trim().isNotEmpty && item.value.trim().isNotEmpty)
        .take(12)
        .toList();

    if (visibleActions.isEmpty) {
      return const SizedBox.shrink();
    }

    return Wrap(
      spacing: 8,
      runSpacing: 8,
      children: visibleActions.map((action) {
        final isPrimary = action.isUploadImage || action.isSelectable;

        return ActionChip(
          avatar: Icon(
            _iconForAction(action),
            size: 16,
            color: isPrimary ? AppColors.primary : AppColors.textMuted,
          ),
          label: Text(
            action.label,
            style: TextStyle(
              fontWeight: FontWeight.w800,
              fontSize: 12.8,
              color: isPrimary ? AppColors.primary : AppColors.textBlack,
            ),
          ),
          backgroundColor: isPrimary ? AppColors.primaryLight : Colors.white,
          side: BorderSide(
            color: isPrimary ? AppColors.strokeStrong : AppColors.stroke,
          ),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(999),
          ),
          onPressed: onSelected == null ? null : () => onSelected!(action),
        );
      }).toList(),
    );
  }

  IconData _iconForAction(ChatAction action) {
    if (action.isUploadImage) {
      return Icons.add_photo_alternate_outlined;
    }

    if (action.type == 'select_symptom') {
      return Icons.check_circle_outline_rounded;
    }

    if (action.type == 'select_severity') {
      return Icons.speed_rounded;
    }

    if (action.type == 'select_growth_stage') {
      return Icons.grass_rounded;
    }

    if (action.isContinueConsultation) {
      return Icons.chat_bubble_outline_rounded;
    }

    return Icons.touch_app_outlined;
  }
}

class _RecommendationCard extends StatelessWidget {
  final ChatMessage message;

  const _RecommendationCard({required this.message});

  @override
  Widget build(BuildContext context) {
    final data = message.recommendation!;

    return Container(
      decoration: BoxDecoration(
        color: AppColors.surfaceSoft,
        borderRadius: BorderRadius.circular(18),
        border: Border.all(color: AppColors.stroke),
      ),
      padding: const EdgeInsets.all(14),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 34,
                height: 34,
                decoration: BoxDecoration(
                  color: AppColors.primary.withOpacity(0.1),
                  borderRadius: BorderRadius.circular(12),
                ),
                child: const Icon(
                  Icons.verified_outlined,
                  size: 18,
                  color: AppColors.primary,
                ),
              ),
              const SizedBox(width: 10),
              const Expanded(
                child: Text(
                  'Ringkasan Diagnosis',
                  style: TextStyle(
                    fontSize: 15,
                    fontWeight: FontWeight.w800,
                    color: AppColors.textBlack,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 12),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              if ((data.pestName ?? '').trim().isNotEmpty)
                _InfoPill(
                  icon: Icons.bug_report_outlined,
                  label: data.pestName!,
                ),
              if ((data.severityName ?? '').trim().isNotEmpty)
                _InfoPill(
                  icon: Icons.speed_rounded,
                  label: 'Keparahan: ${data.severityName}',
                ),
              if ((data.growthStageName ?? '').trim().isNotEmpty)
                _InfoPill(
                  icon: Icons.grass_rounded,
                  label: 'Fase: ${data.growthStageName}',
                ),
              if (data.finalConfidence != null)
                _InfoPill(
                  icon: Icons.analytics_outlined,
                  label: 'Skor: ${_percent(data.finalConfidence!)}',
                ),
            ],
          ),
          if ((data.education ?? '').trim().isNotEmpty) ...[
            const SizedBox(height: 12),
            _SectionText(title: 'Edukasi singkat', text: data.education!),
          ],
          if (data.reasoning.isNotEmpty) ...[
            const SizedBox(height: 12),
            _BulletSection(title: 'Dasar analisis', items: data.reasoning),
          ],
          if (data.prevention.isNotEmpty) ...[
            const SizedBox(height: 12),
            _BulletSection(title: 'Pencegahan', items: data.prevention),
          ],
          if (data.pesticides.isNotEmpty) ...[
            const SizedBox(height: 12),
            _PesticideSection(items: data.pesticides),
          ],
          if (data.warnings.isNotEmpty) ...[
            const SizedBox(height: 12),
            _WarningSection(items: data.warnings),
          ],
          const SizedBox(height: 12),
          const _SafetyNote(),
        ],
      ),
    );
  }

  String _percent(double value) {
    final normalized = value <= 1 ? value * 100 : value;
    return '${normalized.clamp(0, 100).toStringAsFixed(1)}%';
  }
}

class _InfoPill extends StatelessWidget {
  final IconData icon;
  final String label;

  const _InfoPill({required this.icon, required this.label});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(999),
        border: Border.all(color: AppColors.stroke),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 15, color: AppColors.primary),
          const SizedBox(width: 6),
          ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 260),
            child: Text(
              label,
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(
                fontSize: 12.5,
                fontWeight: FontWeight.w700,
                color: AppColors.textBlack,
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _SectionText extends StatelessWidget {
  final String title;
  final String text;

  const _SectionText({required this.title, required this.text});

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _SectionTitle(title),
        const SizedBox(height: 6),
        Text(
          text,
          style: const TextStyle(
            height: 1.55,
            color: AppColors.textMuted,
            fontSize: 13.5,
          ),
        ),
      ],
    );
  }
}

class _BulletSection extends StatelessWidget {
  final String title;
  final List<String> items;

  const _BulletSection({required this.title, required this.items});

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _SectionTitle(title),
        const SizedBox(height: 8),
        ...items.take(3).map(
              (item) => Padding(
                padding: const EdgeInsets.only(bottom: 6),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const Padding(
                      padding: EdgeInsets.only(top: 7),
                      child: Icon(Icons.circle, size: 5, color: AppColors.primary),
                    ),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        item,
                        style: const TextStyle(
                          fontSize: 13.3,
                          height: 1.45,
                          color: AppColors.textMuted,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ),
      ],
    );
  }
}

class _PesticideSection extends StatelessWidget {
  final List<String> items;

  const _PesticideSection({required this.items});

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const _SectionTitle('Rekomendasi pengendalian'),
        const SizedBox(height: 8),
        ...items.take(3).map((item) {
          final parts = item
              .split('\n')
              .where((line) => line.trim().isNotEmpty)
              .toList();
          final rawTitle = parts.isNotEmpty ? parts.first.trim() : item.trim();
          final title = _activeIngredientTitle(rawTitle);
          final productName = _productName(rawTitle);
          final details = parts.length > 1 ? parts.skip(1).toList() : <String>[];

          return Container(
            margin: const EdgeInsets.only(bottom: 8),
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: Colors.white,
              borderRadius: BorderRadius.circular(14),
              border: Border.all(color: AppColors.stroke),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const Padding(
                      padding: EdgeInsets.only(top: 2),
                      child: Icon(
                        Icons.science_outlined,
                        size: 17,
                        color: AppColors.primary,
                      ),
                    ),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            title,
                            style: const TextStyle(
                              fontWeight: FontWeight.w800,
                              fontSize: 13.5,
                              height: 1.3,
                              color: AppColors.textBlack,
                            ),
                          ),
                          if (productName != null) ...[
                            const SizedBox(height: 3),
                            Text(
                              'Produk acuan database: $productName',
                              style: const TextStyle(
                                fontSize: 11.7,
                                height: 1.3,
                                color: AppColors.textMuted,
                                fontStyle: FontStyle.italic,
                              ),
                            ),
                          ],
                        ],
                      ),
                    ),
                  ],
                ),
                if (details.isNotEmpty) ...[
                  const SizedBox(height: 8),
                  ...details.take(4).map(
                        (detail) => Padding(
                          padding: const EdgeInsets.only(bottom: 4),
                          child: Text(
                            _normalizeDetailLine(detail),
                            style: const TextStyle(
                              fontSize: 12.8,
                              height: 1.45,
                              color: AppColors.textMuted,
                            ),
                          ),
                        ),
                      ),
                ],
              ],
            ),
          );
        }),
      ],
    );
  }

  String _activeIngredientTitle(String rawTitle) {
    final text = rawTitle.trim();
    final lower = text.toLowerCase();
    final marker = lower.indexOf(' - bahan aktif ');
    if (marker >= 0) {
      final ingredientPart = text.substring(marker + ' - bahan aktif '.length).trim();
      final cleaned = ingredientPart
          .replaceAll(RegExp(r'\s+-\s+formulasi\s+', caseSensitive: false), ' · formulasi ')
          .replaceAll(RegExp(r'\s+-\s+insektisida\s*$', caseSensitive: false), '')
          .trim();
      if (cleaned.isNotEmpty) {
        return 'Bahan aktif $cleaned';
      }
    }
    return text;
  }

  String? _productName(String rawTitle) {
    final text = rawTitle.trim();
    final marker = text.toLowerCase().indexOf(' - bahan aktif ');
    if (marker <= 0) {
      return null;
    }
    final product = text.substring(0, marker).trim();
    return product.isEmpty ? null : product;
  }

  String _normalizeDetailLine(String value) {
    var text = value.trim().replaceAll(RegExp(r'\s+'), ' ');
    text = text.replaceAll('Dosis: Ikuti dosis pada label produk', 'Dosis: ikuti label resmi produk');
    text = text.replaceAll('Waktu: ', 'Waktu aplikasi: ');
    return text;
  }
}

class _WarningSection extends StatelessWidget {
  final List<String> items;

  const _WarningSection({required this.items});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: const Color(0xFFFFF8E8),
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: const Color(0xFFF4DDA2)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Row(
            children: [
              Icon(Icons.warning_amber_rounded, size: 18, color: AppColors.warning),
              SizedBox(width: 8),
              Text(
                'Catatan keamanan',
                style: TextStyle(
                  fontWeight: FontWeight.w800,
                  color: AppColors.textBlack,
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          ...items.take(3).map(
                (item) => Padding(
                  padding: const EdgeInsets.only(bottom: 4),
                  child: Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      const Padding(
                        padding: EdgeInsets.only(top: 7),
                        child: Icon(
                          Icons.circle,
                          size: 4,
                          color: AppColors.warning,
                        ),
                      ),
                      const SizedBox(width: 8),
                      Expanded(
                        child: Text(
                          item,
                          style: const TextStyle(
                            fontSize: 12.8,
                            height: 1.45,
                            color: AppColors.textMuted,
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
              ),
        ],
      ),
    );
  }
}

class _SafetyNote extends StatelessWidget {
  const _SafetyNote();

  @override
  Widget build(BuildContext context) {
    return const Text(
      'Gunakan pestisida sesuai label resmi, APD, dan validasi kondisi lapangan/penyuluh sebelum aplikasi.',
      style: TextStyle(
        fontSize: 12.2,
        height: 1.45,
        color: AppColors.textMuted,
        fontStyle: FontStyle.italic,
      ),
    );
  }
}

class _SectionTitle extends StatelessWidget {
  final String title;

  const _SectionTitle(this.title);

  @override
  Widget build(BuildContext context) {
    return Text(
      title,
      style: const TextStyle(
        fontSize: 13.5,
        fontWeight: FontWeight.w800,
        color: AppColors.textBlack,
      ),
    );
  }
}
