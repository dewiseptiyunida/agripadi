import 'dart:convert';

import 'package:agripadi/core/network/api_client.dart';
import 'package:agripadi/features/conversation/models/conversation_item.dart';

class ConversationService {
  final ApiClient _apiClient = ApiClient.instance;

  // =========================================================
  // GET USER CONVERSATIONS
  // =========================================================

  Future<List<ConversationItem>> getUserConversations() async {
    try {
      print("========================================");

      print("GET USER CONVERSATIONS");

      final response = await _apiClient.get("/conversations?limit=100", withAuth: true);

      print("GET USER CONVERSATIONS RESPONSE");

      print(jsonEncode(response));

      // =====================================================
      // RESPONSE FORMAT
      //
      // {
      //   "items": [...]
      // }
      //
      // or
      //
      // {
      //   "data": {
      //     "items": [...]
      //   }
      // }
      // =====================================================

      final payload = response["data"] ?? response;

      final List<dynamic> data = payload is Map<String, dynamic>
          ? (payload["items"] ?? [])
          : (payload as List<dynamic>? ?? []);

      final conversations = data
          .map((e) => ConversationItem.fromJson(Map<String, dynamic>.from(e)))
          .toList();

      print("CONVERSATION COUNT: ${conversations.length}");

      print("========================================");

      return conversations;
    } catch (e) {
      print("GET CONVERSATIONS ERROR: $e");

      rethrow;
    }
  }

  // =========================================================
  // DELETE SINGLE CONVERSATION
  // =========================================================

  Future<void> deleteConversation(String conversationId) async {
    try {
      print("========================================");

      print("DELETE CONVERSATION");

      print("CONVERSATION ID: $conversationId");

      final response = await _apiClient.delete(
        "/conversations/$conversationId",
        withAuth: true,
      );

      print("DELETE CONVERSATION RESPONSE");

      print(jsonEncode(response));

      print("========================================");
    } catch (e) {
      print("DELETE CONVERSATION ERROR: $e");

      rethrow;
    }
  }

  // =========================================================
  // DELETE MANY CONVERSATIONS
  // =========================================================

  Future<void> deleteManyConversations(List<String> conversationIds) async {
    try {
      if (conversationIds.isEmpty) {
        return;
      }

      print("========================================");

      print("DELETE MANY CONVERSATIONS");

      print("CONVERSATION COUNT: ${conversationIds.length}");

      print("IDS: $conversationIds");

      final response = await _apiClient.post(
        "/conversations/delete-many",
        body: {"conversation_ids": conversationIds},
        withAuth: true,
      );

      print("DELETE MANY RESPONSE");

      print(jsonEncode(response));

      print("========================================");
    } catch (e) {
      print("DELETE MANY CONVERSATIONS ERROR: $e");

      rethrow;
    }
  }

  // =========================================================
  // DELETE ALL CONVERSATIONS FOR CURRENT USER
  // =========================================================

  Future<int> deleteAllConversations() async {
    try {
      print("========================================");
      print("DELETE ALL CONVERSATIONS");

      final response = await _apiClient.post(
        "/conversations/delete-all",
        withAuth: true,
      );

      print("DELETE ALL RESPONSE");
      print(jsonEncode(response));
      print("========================================");

      final payload = response["data"] ?? response;
      if (payload is Map<String, dynamic>) {
        final deletedCount = payload["deleted_count"];
        if (deletedCount is int) {
          return deletedCount;
        }
        if (deletedCount is num) {
          return deletedCount.toInt();
        }
      }

      return 0;
    } catch (e) {
      print("DELETE ALL CONVERSATIONS ERROR: $e");
      rethrow;
    }
  }

}
