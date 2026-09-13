import 'package:agripadi/core/auth/auth_guard.dart';
import 'package:agripadi/core/network/api_client.dart';
import 'package:flutter/material.dart';
import 'package:image_picker/image_picker.dart';

import '../models/attachment_request.dart';
import '../models/chat_message.dart';
import '../models/send_chat_request.dart';
import '../services/chat_service.dart';

class ChatProvider extends ChangeNotifier {
  final ChatService _chatService = ChatService();

  bool _isLoading = false;
  bool _isTyping = false;
  String? _errorMessage;
  String? _conversationId;

  final List<ChatMessage> _messages = [];

  bool get isLoading => _isLoading;
  bool get isTyping => _isTyping;
  bool get isBusy => _isLoading || _isTyping;
  String? get errorMessage => _errorMessage;
  String? get conversationId => _conversationId;
  List<ChatMessage> get messages => List.unmodifiable(_messages);

  Future<void> initialize() async {
    _errorMessage = null;
    notifyListeners();
  }

  void _setLoading(bool value) {
    _isLoading = value;
    notifyListeners();
  }

  void _setTyping(bool value) {
    _isTyping = value;
    notifyListeners();
  }

  void clearError() {
    _errorMessage = null;
    notifyListeners();
  }

  void setConversationId(String? id) {
    _conversationId = id;
    notifyListeners();
  }

  Future<bool> sendMessage({required String message}) async {
    final text = message.trim();

    if (text.isEmpty || isBusy) {
      return false;
    }

    final pendingId = 'local-${DateTime.now().microsecondsSinceEpoch}';

    try {
      _errorMessage = null;
      _messages.add(
        ChatMessage(
          id: pendingId,
          conversationId: _conversationId ?? '',
          role: 'user',
          type: 'text',
          message: text,
          createdAt: DateTime.now(),
        ),
      );
      _setLoading(true);
      _setTyping(true);

      final response = await _chatService.sendMessage(
        request: SendChatRequest(
          conversationId: _conversationId,
          message: text,
        ),
      );

      _conversationId = response.conversationId;
      _replacePendingUserMessage(
        pendingId,
        ChatMessage.fromChatMessageItem(response.userMessage),
      );
      _messages.add(
        ChatMessage.fromChatMessageItem(
          response.assistantMessage,
          recommendation: response.recommendation,
        ),
      );

      notifyListeners();
      return true;
    } on UnauthorizedException {
      _removeMessageById(pendingId);
      await AuthGuard.handleUnauthorized();
      return false;
    } catch (e) {
      // Jangan biarkan pesan lokal terlihat seolah-olah sudah terkirim.
      _removeMessageById(pendingId);
      _errorMessage = e.toString();
      notifyListeners();
      return false;
    } finally {
      _setTyping(false);
      _setLoading(false);
    }
  }

  Future<bool> loadConversation(String conversationId) async {
    if (isBusy || conversationId.trim().isEmpty) {
      return false;
    }

    final previousConversationId = _conversationId;
    final previousMessages = List<ChatMessage>.from(_messages);

    try {
      _setLoading(true);
      _errorMessage = null;

      _conversationId = conversationId;
      final restoredMessages = await _chatService.getConversationMessages(
        conversationId,
      );

      _messages
        ..clear()
        ..addAll(restoredMessages);
      notifyListeners();
      return true;
    } on UnauthorizedException {
      await AuthGuard.handleUnauthorized();
      return false;
    } catch (e) {
      // Pulihkan tampilan sebelumnya ketika riwayat gagal dimuat.
      _conversationId = previousConversationId;
      _messages
        ..clear()
        ..addAll(previousMessages);
      _errorMessage = 'Gagal memuat percakapan: $e';
      notifyListeners();
      return false;
    } finally {
      _setLoading(false);
    }
  }

  Future<bool> sendImageMessage({
    required XFile imageFile,
    String? caption,
  }) async {
    if (isBusy) {
      return false;
    }

    try {
      _setLoading(true);
      _setTyping(true);
      _errorMessage = null;

      final uploadResult = await _chatService.uploadImage(imageFile);
      if (uploadResult.url.trim().isEmpty) {
        throw const ApiException(
          'Backend tidak mengembalikan alamat gambar yang valid.',
        );
      }
      if (uploadResult.detections.isEmpty) {
        throw const ApiException(
          'Model CNN tidak menghasilkan kandidat klasifikasi.',
        );
      }

      final bytes = await imageFile.readAsBytes();
      final attachment = ChatAttachment(
        type: 'image',
        url: uploadResult.url,
        mimeType: _mimeTypeFor(
          imageFile.name.isNotEmpty ? imageFile.name : imageFile.path,
        ),
        size: bytes.length,
      );

      final request = SendChatRequest(
        conversationId: _conversationId,
        message: caption,
        attachments: [attachment],
        detections: uploadResult.detections,
      );

      final response = await _chatService.sendMessage(request: request);

      _conversationId = response.conversationId;
      _messages.add(ChatMessage.fromChatMessageItem(response.userMessage));
      _messages.add(
        ChatMessage.fromChatMessageItem(
          response.assistantMessage,
          recommendation: response.recommendation,
        ),
      );

      notifyListeners();
      return true;
    } on UnauthorizedException {
      await AuthGuard.handleUnauthorized();
      return false;
    } catch (e) {
      _errorMessage = e.toString();
      notifyListeners();
      return false;
    } finally {
      _setTyping(false);
      _setLoading(false);
    }
  }

  String _mimeTypeFor(String path) {
    final lower = path.toLowerCase();

    if (lower.endsWith('.png')) {
      return 'image/png';
    }
    if (lower.endsWith('.webp')) {
      return 'image/webp';
    }
    return 'image/jpeg';
  }

  void _replacePendingUserMessage(String pendingId, ChatMessage replacement) {
    final index = _messages.indexWhere((item) => item.id == pendingId);

    if (index == -1) {
      _messages.add(replacement);
      return;
    }

    _messages[index] = replacement;
  }

  void _removeMessageById(String messageId) {
    _messages.removeWhere((item) => item.id == messageId);
  }

  void clearChat() {
    if (isBusy) {
      return;
    }

    _messages.clear();
    _conversationId = null;
    _errorMessage = null;
    notifyListeners();
  }
}
