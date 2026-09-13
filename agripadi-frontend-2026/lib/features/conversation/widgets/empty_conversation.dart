import 'package:flutter/material.dart';

class EmptyConversation extends StatelessWidget {
  const EmptyConversation({super.key});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),

        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,

          children: [
            Container(
              width: 90,
              height: 90,

              decoration: BoxDecoration(
                color: const Color(0xFFF1F5F3),

                borderRadius: BorderRadius.circular(24),
              ),

              child: const Icon(
                Icons.chat_bubble_outline,
                size: 44,
                color: Color(0xFF2D6A4F),
              ),
            ),

            const SizedBox(height: 24),

            const Text(
              "Belum Ada Percakapan",
              style: TextStyle(fontSize: 22, fontWeight: FontWeight.bold),
            ),

            const SizedBox(height: 10),

            Text(
              "Mulai percakapan dengan AgriPadi.",
              textAlign: TextAlign.center,
              style: TextStyle(color: Colors.grey.shade600, height: 1.5),
            ),
          ],
        ),
      ),
    );
  }
}
