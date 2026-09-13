import 'package:agripadi/core/constants/app_colors.dart';
import 'package:agripadi/core/network/api_client.dart';
import 'package:agripadi/features/chats/models/attachment_request.dart';
import 'package:flutter/material.dart';

class ImageAttachment extends StatelessWidget {
  final ChatAttachment attachment;

  const ImageAttachment({super.key, required this.attachment});

  @override
  Widget build(BuildContext context) {
    String imageUrl = attachment.url;

    if (!imageUrl.startsWith('http')) {
      imageUrl = '${ApiClient.serverUrl}$imageUrl';
    }

    return ClipRRect(
      borderRadius: BorderRadius.circular(18),
      child: Stack(
        children: [
          AspectRatio(
            aspectRatio: 4 / 3,
            child: Image.network(
              imageUrl,
              fit: BoxFit.cover,
              width: double.infinity,
              loadingBuilder: (context, child, loadingProgress) {
                if (loadingProgress == null) {
                  return child;
                }

                return Container(
                  color: AppColors.surfaceSoft,
                  alignment: Alignment.center,
                  child: const CircularProgressIndicator(color: AppColors.primary),
                );
              },
              errorBuilder: (_, error, stackTrace) {
                return Container(
                  color: AppColors.surfaceSoft,
                  alignment: Alignment.center,
                  child: const Column(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      Icon(Icons.broken_image_outlined, size: 42, color: AppColors.textMuted),
                      SizedBox(height: 8),
                      Text(
                        'Gagal memuat gambar',
                        style: TextStyle(color: AppColors.textMuted),
                      ),
                    ],
                  ),
                );
              },
            ),
          ),
          Positioned(
            left: 10,
            bottom: 10,
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 7),
              decoration: BoxDecoration(
                color: Colors.black.withOpacity(0.48),
                borderRadius: BorderRadius.circular(999),
              ),
              child: const Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(Icons.image_search_rounded, color: Colors.white, size: 15),
                  SizedBox(width: 6),
                  Text(
                    'Gambar scan',
                    style: TextStyle(
                      color: Colors.white,
                      fontSize: 12,
                      fontWeight: FontWeight.w700,
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
