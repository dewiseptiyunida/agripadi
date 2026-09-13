import 'package:agripadi/features/auth/providers/auth_provider.dart';
import 'package:agripadi/features/chats/providers/chat_provider.dart';
import 'package:agripadi/features/conversation/providers/conversation_provider.dart';
import 'package:agripadi/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';

void main() {
  testWidgets('AgriPadi app renders the root MaterialApp', (tester) async {
    await tester.pumpWidget(
      MultiProvider(
        providers: [
          ChangeNotifierProvider(create: (_) => AuthProvider()),
          ChangeNotifierProvider(create: (_) => ChatProvider()),
          ChangeNotifierProvider(create: (_) => ConversationProvider()),
        ],
        child: const MyApp(),
      ),
    );
    await tester.pump(const Duration(seconds: 3));

    expect(find.byType(MaterialApp), findsOneWidget);
  });
}
