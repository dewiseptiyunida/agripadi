import 'package:agripadi/features/chats/models/chat_message.dart';

class ConversationItem {
  final String id;
  final String title;
  final String mode;
  final String state;
  final String lastMessage;
  final String? lastMessageRole;
  final String? lastMessageType;
  final DateTime? lastMessageAt;
  final int unreadCount;
  final DateTime createdAt;
  final DateTime updatedAt;

  const ConversationItem({
    required this.id,
    required this.title,
    required this.mode,
    required this.state,
    required this.lastMessage,
    this.lastMessageRole,
    this.lastMessageType,
    this.lastMessageAt,
    required this.unreadCount,
    required this.createdAt,
    required this.updatedAt,
  });

  bool get isImageMessage => lastMessageType == "image";

  factory ConversationItem.fromJson(Map<String, dynamic> json) {
    return ConversationItem(
      id: json["id"]?.toString() ?? "",
      title: json["title"]?.toString() ?? "Percakapan Baru",
      mode: json["mode"]?.toString() ?? "consultation",
      state: json["state"]?.toString() ?? "idle",
      lastMessage: json["last_message"]?.toString() ?? "",
      lastMessageRole: json["last_message_role"]?.toString(),
      lastMessageType: json["last_message_type"]?.toString(),
      lastMessageAt: _parseNullableDate(json["last_message_at"]),
      unreadCount: _asInt(json["unread_count"]),
      createdAt: _parseDate(json["created_at"]),
      updatedAt: _parseDate(json["updated_at"]),
    );
  }

  static DateTime _parseDate(dynamic value) {
    return DateTime.tryParse(value?.toString() ?? "") ?? DateTime.now();
  }

  static DateTime? _parseNullableDate(dynamic value) {
    if (value == null) {
      return null;
    }

    return DateTime.tryParse(value.toString());
  }

  static int _asInt(dynamic value) {
    if (value is int) {
      return value;
    }

    if (value is num) {
      return value.toInt();
    }

    return int.tryParse(value?.toString() ?? "") ?? 0;
  }
}

class ConversationDetail {
  final ConversationItem conversation;
  final List<ChatMessageItem> messages;

  const ConversationDetail({
    required this.conversation,
    this.messages = const [],
  });

  factory ConversationDetail.fromJson(Map<String, dynamic> json) {
    final messages = json["messages"] is List
        ? (json["messages"] as List)
              .whereType<Map>()
              .map(
                (item) =>
                    ChatMessageItem.fromJson(Map<String, dynamic>.from(item)),
              )
              .toList()
        : <ChatMessageItem>[];

    return ConversationDetail(
      conversation: ConversationItem.fromJson(json),
      messages: messages,
    );
  }
}
