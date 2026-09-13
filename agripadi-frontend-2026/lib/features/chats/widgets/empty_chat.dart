import 'package:agripadi/core/constants/app_colors.dart';
import 'package:agripadi/core/network/api_client.dart';
import 'package:flutter/material.dart';

class EmptyChat extends StatelessWidget {
  final ValueChanged<String>? onPromptSelected;

  const EmptyChat({super.key, this.onPromptSelected});

  @override
  Widget build(BuildContext context) {
    final prompts = <_PromptItem>[
      const _PromptItem(
        icon: Icons.camera_alt_outlined,
        title: 'Analisis hasil scan hama padi',
        prompt: 'Saya ingin menganalisis hama padi dari gambar dan mendapatkan rekomendasi pengendalian yang aman.',
      ),
      const _PromptItem(
        icon: Icons.shield_outlined,
        title: 'Minta rekomendasi pestisida aman',
        prompt: 'Bagaimana rekomendasi pengendalian hama padi yang aman dan tidak berlebihan menggunakan pestisida?',
      ),
      const _PromptItem(
        icon: Icons.eco_outlined,
        title: 'Konsultasi gejala tanaman',
        prompt: 'Tanaman padi saya menunjukkan gejala kerusakan. Bantu saya melakukan konsultasi gejala secara bertahap.',
      ),
    ];

    return LayoutBuilder(
      builder: (_, constraints) {
        return SingleChildScrollView(
          physics: const BouncingScrollPhysics(),
          child: ConstrainedBox(
            constraints: BoxConstraints(minHeight: constraints.maxHeight),
            child: Padding(
              padding: const EdgeInsets.fromLTRB(20, 18, 20, 24),
              child: Center(
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 760),
                  child: Column(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      Container(
                        padding: const EdgeInsets.all(18),
                        decoration: BoxDecoration(
                          gradient: AppColors.primaryGradient,
                          borderRadius: BorderRadius.circular(30),
                          boxShadow: [
                            BoxShadow(
                              color: AppColors.primary.withOpacity(0.22),
                              blurRadius: 28,
                              offset: const Offset(0, 16),
                            ),
                          ],
                        ),
                        child: const Icon(
                          Icons.chat_bubble_rounded,
                          size: 42,
                          color: Colors.white,
                        ),
                      ),
                      const SizedBox(height: 22),
                      const Text(
                        'AgriPadi Chat Assistant',
                        textAlign: TextAlign.center,
                        style: TextStyle(
                          fontSize: 28,
                          fontWeight: FontWeight.w900,
                          letterSpacing: -0.6,
                          color: AppColors.textBlack,
                        ),
                      ),
                      const SizedBox(height: 10),
                      Text(
                        'Konsultasi hama padi, analisis gambar CNN MobileNetV3-Small on-device, dan rekomendasi pengendalian berbasis sistem pakar + LLM dalam satu alur chat.',
                        textAlign: TextAlign.center,
                        style: TextStyle(
                          color: Colors.grey.shade700,
                          fontSize: 15,
                          height: 1.6,
                        ),
                      ),
                      const SizedBox(height: 18),
                      const SizedBox(height: 26),
                      GridView.builder(
                        shrinkWrap: true,
                        physics: const NeverScrollableScrollPhysics(),
                        gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
                          crossAxisCount: constraints.maxWidth < 620 ? 1 : 3,
                          mainAxisExtent: 126,
                          crossAxisSpacing: 12,
                          mainAxisSpacing: 12,
                        ),
                        itemCount: prompts.length,
                        itemBuilder: (context, index) {
                          final item = prompts[index];
                          return _PromptCard(
                            item: item,
                            onTap: onPromptSelected == null
                                ? null
                                : () => onPromptSelected!(item.prompt),
                          );
                        },
                      ),
                      const SizedBox(height: 18),
                      const Text(
                        'Tip: tekan tombol gambar di kolom chat untuk mengambil foto langsung dari kamera atau memilih dari galeri.',
                        textAlign: TextAlign.center,
                        style: TextStyle(
                          color: AppColors.textMuted,
                          fontSize: 12.5,
                          height: 1.5,
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ),
          ),
        );
      },
    );
  }
}

class _PromptItem {
  final IconData icon;
  final String title;
  final String prompt;

  const _PromptItem({
    required this.icon,
    required this.title,
    required this.prompt,
  });
}

class _PromptCard extends StatelessWidget {
  final _PromptItem item;
  final VoidCallback? onTap;

  const _PromptCard({required this.item, this.onTap});

  @override
  Widget build(BuildContext context) {
    return Material(
      color: Colors.white,
      borderRadius: BorderRadius.circular(22),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(22),
        child: Container(
          padding: const EdgeInsets.all(16),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(22),
            border: Border.all(color: AppColors.stroke),
            boxShadow: [
              BoxShadow(
                color: Colors.black.withOpacity(0.035),
                blurRadius: 16,
                offset: const Offset(0, 8),
              ),
            ],
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Container(
                width: 38,
                height: 38,
                decoration: BoxDecoration(
                  color: AppColors.primaryLight,
                  borderRadius: BorderRadius.circular(14),
                ),
                child: Icon(item.icon, color: AppColors.primary, size: 20),
              ),
              const Spacer(),
              Text(
                item.title,
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(
                  fontSize: 14,
                  height: 1.25,
                  fontWeight: FontWeight.w800,
                  color: AppColors.textBlack,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
