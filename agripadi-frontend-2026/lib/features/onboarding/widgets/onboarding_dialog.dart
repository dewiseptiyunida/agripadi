import 'package:flutter/material.dart';

class OnboardingDialog extends StatefulWidget {
  const OnboardingDialog({
    super.key,
    required this.onFinished,
  });

  final Future<void> Function() onFinished;

  @override
  State<OnboardingDialog> createState() =>
      _OnboardingDialogState();
}

class _OnboardingDialogState
    extends State<OnboardingDialog> {
  int _currentPage = 0;

  final List<_TutorialStep> _steps = [
    _TutorialStep(
      icon: Icons.waving_hand_rounded,
      title: 'Selamat Datang di AgriPadi',
      description:
          'AgriPadi membantu petani melakukan konsultasi budidaya padi, diagnosis hama berbasis AI, serta mendapatkan rekomendasi pengendalian yang sesuai.\n\nTekan Lanjut untuk melihat cara penggunaan aplikasi.',
    ),

    _TutorialStep(
      icon: Icons.chat_bubble_outline,
      title: 'Konsultasi AI',
      description:
          'Anda dapat bertanya seputar budidaya padi, pemupukan, pengairan, maupun pengendalian hama.\n\nAsisten AI akan memberikan penjelasan dan saran yang mudah dipahami.',
    ),

    _TutorialStep(
      icon: Icons.camera_alt_outlined,
      title: 'Diagnosis Hama',
      description:
          'Jika menemukan hama pada tanaman padi, unggah foto hama atau gunakan kamera.\n\nPastikan hama terlihat jelas, tidak terlalu jauh, dan pencahayaan cukup.',
    ),

    _TutorialStep(
      icon: Icons.search,
      title: 'Identifikasi Otomatis',
      description:
          'Sistem AI akan menganalisis gambar dan mengenali jenis hama yang terdeteksi.\n\nHasil identifikasi digunakan sebagai dasar proses diagnosis berikutnya.',
    ),

    _TutorialStep(
      icon: Icons.assignment_outlined,
      title: 'Informasi Gejala',
      description:
          'Setelah hama teridentifikasi, Anda akan diminta menjelaskan gejala yang terlihat pada tanaman.\n\nInformasi ini membantu meningkatkan ketepatan diagnosis.',
    ),

    _TutorialStep(
      icon: Icons.warning_amber_rounded,
      title: 'Tingkat Keparahan',
      description:
          'Pilih tingkat keparahan serangan hama sesuai kondisi lahan.\n\nPilihan ini membantu sistem menentukan tindakan pengendalian yang sesuai.',
    ),

    _TutorialStep(
      icon: Icons.grass,
      title: 'Fase Pertumbuhan',
      description:
          'Pilih fase pertumbuhan tanaman padi saat ini.\n\nFase pertumbuhan digunakan untuk menyesuaikan rekomendasi pengendalian.',
    ),

    _TutorialStep(
      icon: Icons.analytics_outlined,
      title: 'Hasil Diagnosis',
      description:
          'Sistem akan menampilkan jenis hama, tingkat keyakinan deteksi, tingkat keparahan, dan ringkasan diagnosis.',
    ),

    _TutorialStep(
      icon: Icons.science_outlined,
      title: 'Rekomendasi Pengendalian',
      description:
          'AgriPadi akan memberikan rekomendasi pestisida, alasan pemilihan, serta tindakan pengendalian berdasarkan kondisi serangan.',
    ),
  ];

  Future<void> _finish() async {
    await widget.onFinished();

    if (!mounted) {
      return;
    }

    Navigator.pop(context);
  }

  Future<void> _next() async {
    if (_currentPage == _steps.length - 1) {
      await _finish();
      return;
    }

    setState(() {
      _currentPage++;
    });
  }

  void _previous() {
    if (_currentPage == 0) {
      return;
    }

    setState(() {
      _currentPage--;
    });
  }

  @override
  Widget build(BuildContext context) {
    final step = _steps[_currentPage];

    return Dialog(
      insetPadding: const EdgeInsets.all(20),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(28),
      ),
      child: ConstrainedBox(
        constraints: const BoxConstraints(
          maxWidth: 550,
          maxHeight: 700,
        ),
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Row(
                  children: [
                    if (_currentPage == 0)
                      TextButton(
                        onPressed: _finish,
                        child: const Text(
                          "Lewati",
                        ),
                      ),

                    const Spacer(),

                    IconButton(
                      icon: const Icon(
                        Icons.close,
                      ),
                      onPressed: () {
                        Navigator.pop(context);
                      },
                    ),
                  ],
                ),

                const SizedBox(height: 8),

                Container(
                  width: 120,
                  height: 120,
                  decoration: BoxDecoration(
                    color: Colors.green.shade50,
                    shape: BoxShape.circle,
                  ),
                  child: Icon(
                    step.icon,
                    size: 64,
                    color: Colors.green.shade700,
                  ),
                ),

                const SizedBox(height: 24),

                Text(
                  "${_currentPage + 1}/${_steps.length}",
                  style: TextStyle(
                    color: Colors.green.shade700,
                    fontWeight: FontWeight.w700,
                    fontSize: 14,
                  ),
                ),

                const SizedBox(height: 12),

                Text(
                  step.title,
                  textAlign: TextAlign.center,
                  style: const TextStyle(
                    fontSize: 24,
                    fontWeight: FontWeight.w800,
                  ),
                ),

                const SizedBox(height: 16),

                Text(
                  step.description,
                  textAlign: TextAlign.center,
                  style: TextStyle(
                    height: 1.6,
                    color: Colors.grey.shade700,
                    fontSize: 15,
                  ),
                ),

                const SizedBox(height: 32),

                Row(
                  mainAxisAlignment:
                      MainAxisAlignment.center,
                  children: List.generate(
                    _steps.length,
                    (index) {
                      final active =
                          index == _currentPage;

                      return AnimatedContainer(
                        duration:
                            const Duration(
                          milliseconds: 250,
                        ),
                        margin:
                            const EdgeInsets.symmetric(
                          horizontal: 4,
                        ),
                        width:
                            active ? 24 : 8,
                        height: 8,
                        decoration:
                            BoxDecoration(
                          color: active
                              ? Colors.green
                              : Colors.grey.shade300,
                          borderRadius:
                              BorderRadius.circular(
                            99,
                          ),
                        ),
                      );
                    },
                  ),
                ),

                const SizedBox(height: 32),

                Row(
                  children: [
                    if (_currentPage > 0)
                      Expanded(
                        child: OutlinedButton(
                          onPressed:
                              _previous,
                          child: const Text(
                            "Kembali",
                          ),
                        ),
                      ),

                    if (_currentPage > 0)
                      const SizedBox(
                        width: 12,
                      ),

                    Expanded(
                      child: FilledButton(
                        onPressed:
                            _next,
                        child: Text(
                          _currentPage ==
                                  _steps.length -
                                      1
                              ? "Selesai"
                              : "Lanjut",
                        ),
                      ),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _TutorialStep {
  final IconData icon;
  final String title;
  final String description;

  const _TutorialStep({
    required this.icon,
    required this.title,
    required this.description,
  });
}