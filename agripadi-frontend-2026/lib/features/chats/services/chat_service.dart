import 'dart:convert';

import 'package:agripadi/core/network/api_client.dart';
import 'package:agripadi/features/conversation/models/conversation_item.dart';
import 'package:agripadi/features/cnn/services/on_device_cnn_service.dart';
import 'package:image_picker/image_picker.dart';

import '../models/chat_message.dart';
import '../models/send_chat_request.dart';
import '../models/send_chat_response.dart';

class ChatService {
  final ApiClient _apiClient = ApiClient.instance;

  // =========================================================
  // GET CONVERSATION MESSAGES
  // =========================================================

  Future<List<ChatMessage>> getConversationMessages(
    String conversationId,
  ) async {
    try {
      print("========================================");

      print("GET CONVERSATION MESSAGES");

      print("CONVERSATION ID: $conversationId");

      final response = await _apiClient.get(
        "/conversations/$conversationId/messages",
        withAuth: true,
      );

      print("GET CONVERSATION MESSAGES RESPONSE");

      print(jsonEncode(response));

      // =====================================================
      // RESPONSE FORMAT
      // {
      //   "items": [...]
      // }
      //
      // or
      //
      // {
      //   "data": [...]
      // }
      // =====================================================

      final List<dynamic> rawItems =
          response["items"] ?? response["data"] ?? [];

      final messages = rawItems
          .map(
            (item) => ChatMessage.fromChatMessageItem(
              ChatMessageItem.fromJson(Map<String, dynamic>.from(item)),
            ),
          )
          .toList();

      print("MESSAGE COUNT: ${messages.length}");

      print("========================================");

      return messages;
    } catch (e) {
      print("GET CONVERSATION MESSAGES ERROR: $e");

      rethrow;
    }
  }

  // =========================================================
  // SEND MESSAGE
  // =========================================================

  Future<SendChatResponse> sendMessage({
    required SendChatRequest request,
  }) async {
    try {
      print("========================================");

      print("CHAT SEND MESSAGE REQUEST");

      print(jsonEncode(request.toJson()));

      print("MESSAGE: ${request.message}");

      print("CONVERSATION ID: ${request.conversationId}");

      // =====================================================
      // ENSURE CONVERSATION
      // =====================================================

      final conversationId = await _ensureConversationId(
        request.conversationId,
      );

      // =====================================================
      // SEND REQUEST
      // =====================================================

      final response = await _apiClient.post(
        "/conversations/$conversationId/messages",

        body: {...request.toJson(), "conversation_id": conversationId},
        withAuth: true,
      );

      print("CHAT SEND MESSAGE RESPONSE");

      print(jsonEncode(response));

      final data = response["data"] ?? response;

      // =====================================================
      // RESPONSE PARSING
      // =====================================================

      final result =
          data is Map<String, dynamic> && data.containsKey("user_message")
          ? SendChatResponse.fromJson(data)
          : SendChatResponse.fromOrchestrator(
              json: Map<String, dynamic>.from(data),
              request: request,
              conversationId: conversationId,
            );

      // =====================================================
      // METADATA
      // =====================================================

      if (result.metadata != null) {
        print("MODEL: ${result.metadata!.model}");

        print("TOKENS USED: ${result.metadata!.tokensUsed}");

        print("LATENCY: ${result.metadata!.latencyMS}ms");
      }

      // =====================================================
      // RECOMMENDATION
      // =====================================================

      if (result.recommendation != null) {
        print("RECOMMENDATION DETECTED");

        print("PESTICIDES: ${result.recommendation!.pesticides}");

        print("WARNINGS: ${result.recommendation!.warnings}");
      }

      print("========================================");

      return result;
    } catch (e) {
      print("CHAT SEND MESSAGE ERROR: $e");

      rethrow;
    }
  }

  // =========================================================
  // UPLOAD IMAGE
  // =========================================================

  Future<UploadImageResult> uploadImage(XFile imageFile) async {
    try {
      print("========================================");

      print("CHAT UPLOAD IMAGE REQUEST");

      print("FILE PATH: ${imageFile.path}");

      print("FILE NAME: ${imageFile.name}");

      final bytes = await imageFile.readAsBytes();

      print("FILE SIZE: ${bytes.length} bytes");

      final List<DetectionCandidate> localDetections;
      try {
        localDetections = await OnDeviceCnnService.instance.classifyImage(
          imageFile,
        );
      } catch (error) {
        throw ApiException(
          "Model deteksi hama di perangkat belum siap atau gambar tidak dapat dianalisis. Pastikan aset model tersedia dan gunakan foto hama yang jelas.",
        );
      }

      print("ON DEVICE CNN DETECTIONS");
      print(jsonEncode(localDetections.map((item) => item.toJson()).toList()));

      final response = await _apiClient.uploadImage(
        file: imageFile,
        skipServerCnn: true,
        withAuth: true,
      );

      print("CHAT UPLOAD IMAGE RESPONSE");

      print(jsonEncode(response));

      final result = UploadImageResult.fromJson(
        Map<String, dynamic>.from(response as Map),
      );

      final mergedResult = result.copyWith(
        detections: localDetections.isNotEmpty
            ? localDetections
            : result.detections,
      );

      print("UPLOADED IMAGE URL: ${mergedResult.url}");
      print("CNN DETECTION COUNT: ${mergedResult.detections.length}");

      print("========================================");

      return mergedResult;
    } catch (e) {
      print("CHAT UPLOAD IMAGE ERROR: $e");

      rethrow;
    }
  }

  // =========================================================
  // CREATE CONVERSATION IF NEEDED
  // =========================================================

  Future<String> _ensureConversationId(String? conversationId) async {
    if (conversationId != null && conversationId.isNotEmpty) {
      return conversationId;
    }

    print("========================================");

    print("CREATE NEW CONVERSATION");

    final response = await _apiClient.post("/conversations", withAuth: true);

    print("CREATE CONVERSATION RESPONSE");

    print(jsonEncode(response));

    final payload = response["data"] ?? response;

    final item = ConversationItem.fromJson(payload);

    if (item.id.isEmpty) {
      throw Exception("Backend tidak mengembalikan conversation id");
    }

    print("NEW CONVERSATION ID: ${item.id}");

    print("========================================");

    return item.id;
  }
}

class UploadImageResult {
  final String url;
  final List<DetectionCandidate> detections;

  const UploadImageResult({required this.url, this.detections = const []});

  UploadImageResult copyWith({
    String? url,
    List<DetectionCandidate>? detections,
  }) {
    return UploadImageResult(
      url: url ?? this.url,
      detections: detections ?? this.detections,
    );
  }

  factory UploadImageResult.fromJson(Map<String, dynamic> json) {
    final rawDetections = json["detections"];
    final detections = rawDetections is List
        ? rawDetections
              .whereType<Map>()
              .map(
                (item) => DetectionCandidate.fromJson(
                  Map<String, dynamic>.from(item),
                ),
              )
              .where((item) => item.label.isNotEmpty && item.model.isNotEmpty)
              .toList()
        : <DetectionCandidate>[];

    return UploadImageResult(
      url: json["url"]?.toString() ?? "",
      detections: detections,
    );
  }
}
