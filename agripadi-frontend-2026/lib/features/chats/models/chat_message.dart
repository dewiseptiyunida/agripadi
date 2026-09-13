import 'attachment_request.dart';
import 'chat_action.dart';
import 'recommendation_data.dart';

class ChatMessage {
  final String id;
  final String conversationId;
  final String role;
  final String type;
  final String message;
  final DateTime createdAt;
  final List<ChatAttachment> attachments;
  final RecommendationData? recommendation;
  final List<ChatAction> actions;

  const ChatMessage({
    required this.id,
    required this.conversationId,
    required this.role,
    required this.type,
    required this.message,
    required this.createdAt,
    this.attachments = const [],
    this.recommendation,
    this.actions = const [],
  });

  bool get isUser => role.toLowerCase() == "user";

  bool get hasAttachment => attachments.isNotEmpty;

  bool get hasRecommendation =>
      recommendation != null && !recommendation!.isEmpty;

  bool get hasActions => actions.isNotEmpty;

  factory ChatMessage.fromChatMessageItem(
    ChatMessageItem item, {
    RecommendationData? recommendation,
  }) {
    return ChatMessage(
      id: item.id,
      conversationId: item.conversationId,
      role: item.role,
      type: item.type,
      message: item.content,
      createdAt: item.createdAt,
      attachments: item.attachments,
      recommendation: recommendation ?? item.recommendation,
      actions: item.actions,
    );
  }
}

class ChatMessageItem {
  final String id;
  final String conversationId;
  final String role;
  final String type;
  final String content;
  final String status;
  final bool isStreaming;
  final DateTime createdAt;
  final List<ChatAttachment> attachments;
  final AIResponseMetadata? ai;
  final RecommendationData? recommendation;
  final List<ChatAction> actions;

  const ChatMessageItem({
    required this.id,
    required this.conversationId,
    required this.role,
    required this.type,
    required this.content,
    required this.status,
    required this.isStreaming,
    required this.createdAt,
    this.attachments = const [],
    this.ai,
    this.recommendation,
    this.actions = const [],
  });

  factory ChatMessageItem.fromJson(Map<String, dynamic> json) {
    final attachments = json["attachments"] is List
        ? (json["attachments"] as List)
              .whereType<Map>()
              .map(
                (item) =>
                    ChatAttachment.fromJson(Map<String, dynamic>.from(item)),
              )
              .toList()
        : <ChatAttachment>[];

    final actions = json["actions"] is List
        ? (json["actions"] as List)
              .whereType<Map>()
              .map((item) => ChatAction.fromJson(Map<String, dynamic>.from(item)))
              .where((item) => item.label.trim().isNotEmpty && item.value.trim().isNotEmpty)
              .toList()
        : <ChatAction>[];

    final recommendation = RecommendationData.fromMessageJson(json);

    return ChatMessageItem(
      id: json["id"]?.toString() ?? "",
      conversationId: json["conversation_id"]?.toString() ?? "",
      role: (json["role"] ?? json["sender"] ?? "assistant").toString(),
      type: (json["type"] ?? json["chat_type"] ?? "text").toString(),
      content: (json["content"] ?? json["message"] ?? "").toString(),
      status: json["status"]?.toString() ?? "sent",
      isStreaming: json["is_streaming"] == true,
      createdAt: _parseDate(json["created_at"]),
      attachments: attachments,
      ai: json["ai"] is Map
          ? AIResponseMetadata.fromJson(
              Map<String, dynamic>.from(json["ai"] as Map),
            )
          : null,
      recommendation: recommendation.isEmpty ? null : recommendation,
      actions: actions,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      "id": id,
      "conversation_id": conversationId,
      "role": role,
      "type": type,
      "content": content,
      "status": status,
      "is_streaming": isStreaming,
      "created_at": createdAt.toIso8601String(),
      if (attachments.isNotEmpty)
        "attachments": attachments.map((item) => item.toJson()).toList(),
      if (ai != null) "ai": ai!.toJson(),
      if (recommendation != null && !recommendation!.isEmpty)
        "payload": recommendation!.toPayloadJson(),
      if (actions.isNotEmpty)
        "actions": actions.map((item) => item.toJson()).toList(),
    };
  }

  static DateTime _parseDate(dynamic value) {
    return DateTime.tryParse(value?.toString() ?? "") ?? DateTime.now();
  }
}

class AIResponseMetadata {
  final String? provider;
  final String? model;
  final int? latencyMS;
  final int? inputTokens;
  final int? outputTokens;
  final int? totalTokens;
  final String? finishReason;

  const AIResponseMetadata({
    this.provider,
    this.model,
    this.latencyMS,
    this.inputTokens,
    this.outputTokens,
    this.totalTokens,
    this.finishReason,
  });

  int? get tokensUsed => totalTokens;

  factory AIResponseMetadata.fromJson(Map<String, dynamic> json) {
    return AIResponseMetadata(
      provider: json["provider"]?.toString(),
      model: json["model"]?.toString(),
      latencyMS: _nullableInt(json["latency_ms"]),
      inputTokens: _nullableInt(json["input_tokens"]),
      outputTokens: _nullableInt(json["output_tokens"]),
      totalTokens: _nullableInt(json["total_tokens"]),
      finishReason: json["finish_reason"]?.toString(),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      if (provider != null) "provider": provider,
      if (model != null) "model": model,
      if (latencyMS != null) "latency_ms": latencyMS,
      if (inputTokens != null) "input_tokens": inputTokens,
      if (outputTokens != null) "output_tokens": outputTokens,
      if (totalTokens != null) "total_tokens": totalTokens,
      if (finishReason != null) "finish_reason": finishReason,
    };
  }

  static int? _nullableInt(dynamic value) {
    if (value == null) {
      return null;
    }

    if (value is int) {
      return value;
    }

    if (value is num) {
      return value.toInt();
    }

    return int.tryParse(value.toString());
  }
}
