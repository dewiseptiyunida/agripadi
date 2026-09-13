import 'package:uuid/uuid.dart';

import 'chat_action.dart';
import 'chat_message.dart';
import 'recommendation_data.dart';
import 'send_chat_request.dart';

class SendChatResponse {
  final String conversationId;
  final ChatMessageItem userMessage;
  final ChatMessageItem assistantMessage;
  final AIResponseMetadata? metadata;
  final RecommendationData? recommendation;

  const SendChatResponse({
    required this.conversationId,
    required this.userMessage,
    required this.assistantMessage,
    this.metadata,
    this.recommendation,
  });

  factory SendChatResponse.fromJson(Map<String, dynamic> json) {
    final userMessage = ChatMessageItem.fromJson(
      Map<String, dynamic>.from(json["user_message"] as Map),
    );
    final assistantJson = json["assistant_message"] ?? json["ai_message"];
    final assistantMessage = ChatMessageItem.fromJson(
      Map<String, dynamic>.from(assistantJson as Map),
    );

    return SendChatResponse(
      conversationId:
          json["conversation_id"]?.toString() ?? userMessage.conversationId,
      userMessage: userMessage,
      assistantMessage: assistantMessage,
      metadata: assistantMessage.ai,
      recommendation: assistantMessage.recommendation,
    );
  }

  factory SendChatResponse.fromOrchestrator({
    required Map<String, dynamic> json,
    required SendChatRequest request,
    required String conversationId,
  }) {
    const uuid = Uuid();
    final now = DateTime.now();
    final createdAt =
        DateTime.tryParse(json["created_at"]?.toString() ?? "") ?? now;
    final recommendation = RecommendationData.fromOrchestrator(json);
    final actions = json["actions"] is List
        ? (json["actions"] as List)
              .whereType<Map>()
              .map((item) => ChatAction.fromJson(Map<String, dynamic>.from(item)))
              .where((item) => item.label.trim().isNotEmpty && item.value.trim().isNotEmpty)
              .toList()
        : <ChatAction>[];

    final userMessage = ChatMessageItem(
      id: uuid.v4(),
      conversationId: conversationId,
      role: "user",
      type: request.hasImage ? "image" : "text",
      content: request.content,
      status: "sent",
      isStreaming: false,
      createdAt: now,
      attachments: request.attachments,
    );

    final assistantMessage = ChatMessageItem(
      id: uuid.v4(),
      conversationId: conversationId,
      role: "assistant",
      type: "text",
      content: json["message"]?.toString() ?? "",
      status: "sent",
      isStreaming: false,
      createdAt: createdAt,
      actions: actions,
      recommendation: recommendation.isEmpty ? null : recommendation,
    );

    return SendChatResponse(
      conversationId: conversationId,
      userMessage: userMessage,
      assistantMessage: assistantMessage,
      recommendation: recommendation.isEmpty ? null : recommendation,
    );
  }
}
