import 'package:agripadi/core/auth/auth_guard.dart';
import 'package:agripadi/core/network/api_client.dart';
import 'package:flutter/material.dart';

import '../models/conversation_item.dart';
import '../services/conversation_service.dart';

class ConversationProvider extends ChangeNotifier {
  final ConversationService _conversationService = ConversationService();

  // =========================================================
  // STATE
  // =========================================================

  bool _isLoading = false;

  bool _isDeleting = false;

  String? _errorMessage;

  List<ConversationItem> _conversations = [];

  ConversationItem? _selectedConversation;

  // =========================================================
  // GETTER
  // =========================================================

  bool get isLoading => _isLoading;

  bool get isDeleting => _isDeleting;

  String? get errorMessage => _errorMessage;

  List<ConversationItem> get conversations => _conversations;

  ConversationItem? get selectedConversation => _selectedConversation;

  // =========================================================
  // SET LOADING
  // =========================================================

  void _setLoading(bool value) {
    _isLoading = value;

    notifyListeners();
  }

  // =========================================================
  // SET DELETING
  // =========================================================

  void _setDeleting(bool value) {
    _isDeleting = value;

    notifyListeners();
  }

  // =========================================================
  // CLEAR ERROR
  // =========================================================

  void clearError() {
    _errorMessage = null;

    notifyListeners();
  }

  // =========================================================
  // SELECT CONVERSATION
  // =========================================================

  void selectConversation(ConversationItem conversation) {
    _selectedConversation = conversation;

    notifyListeners();
  }


  // =========================================================
  // START NEW CONVERSATION LOCAL STATE
  // =========================================================

  void startNewConversation() {
    _selectedConversation = null;
    _errorMessage = null;

    notifyListeners();
  }

  // =========================================================
  // SELECT CONVERSATION BY ID
  // =========================================================

  void selectConversationById(String? conversationId) {
    if (conversationId == null || conversationId.isEmpty) {
      startNewConversation();
      return;
    }

    final index = _conversations.indexWhere((item) => item.id == conversationId);

    if (index == -1) {
      return;
    }

    _selectedConversation = _conversations[index];

    notifyListeners();
  }

  // =========================================================
  // GET CONVERSATIONS
  // =========================================================

  Future<void> getUserConversations() async {
    try {
      _setLoading(true);

      _errorMessage = null;

      final result = await _conversationService.getUserConversations();

      _conversations = result;

      // =====================================================
      // PRESERVE SELECTED CONVERSATION ONLY IF IT STILL EXISTS
      // =====================================================

      if (_selectedConversation != null &&
          !_conversations.any((item) => item.id == _selectedConversation!.id)) {
        _selectedConversation = null;
      }

      notifyListeners();
    } catch (e) {
      if (e is UnauthorizedException) {
        await AuthGuard.handleUnauthorized();
        return;
      }

      _errorMessage = e.toString();
    } finally {
      _setLoading(false);
    }
  }

  // =========================================================
  // REFRESH
  // =========================================================

  Future<void> refresh() async {
    await getUserConversations();
  }

  // =========================================================
  // ADD CONVERSATION
  // =========================================================

  void addConversation(ConversationItem conversation) {
    _conversations.insert(0, conversation);

    notifyListeners();
  }

  // =========================================================
  // UPDATE CONVERSATION
  // =========================================================

  void updateConversation(ConversationItem updated) {
    final index = _conversations.indexWhere((e) => e.id == updated.id);

    if (index != -1) {
      _conversations[index] = updated;

      notifyListeners();
    }
  }

  // =========================================================
  // UPSERT CONVERSATION
  // =========================================================

  void upsertConversation(ConversationItem conversation) {
    final index = _conversations.indexWhere((e) => e.id == conversation.id);

    // =====================================================
    // EXISTING
    // =====================================================

    if (index != -1) {
      _conversations.removeAt(index);

      _conversations.insert(0, conversation);
    }
    // =====================================================
    // NEW
    // =====================================================
    else {
      _conversations.insert(0, conversation);
    }

    _selectedConversation = conversation;

    notifyListeners();
  }

  // =========================================================
  // DELETE SINGLE CONVERSATION
  // =========================================================

  Future<bool> deleteConversation(String conversationId) async {
    try {
      _setDeleting(true);

      _errorMessage = null;

      print("========================================");
      print("DELETE CONVERSATION");
      print("CONVERSATION ID: $conversationId");

      await _conversationService.deleteConversation(conversationId);

      _conversations.removeWhere((e) => e.id == conversationId);

      if (_selectedConversation?.id == conversationId) {
        _selectedConversation = _conversations.isNotEmpty
            ? _conversations.first
            : null;
      }

      notifyListeners();

      print("DELETE CONVERSATION SUCCESS");
      print("========================================");
      return true;
    } catch (e) {
      print("DELETE CONVERSATION ERROR: $e");

      if (e is UnauthorizedException) {
        await AuthGuard.handleUnauthorized();
        return false;
      }

      _errorMessage = "Gagal menghapus percakapan";

      notifyListeners();
      return false;
    } finally {
      _setDeleting(false);
    }
  }

  // =========================================================
  // DELETE MANY CONVERSATIONS
  // =========================================================

  Future<bool> deleteManyConversations(List<String> conversationIds) async {
    try {
      if (conversationIds.isEmpty) {
        return true;
      }

      _setDeleting(true);

      _errorMessage = null;

      print("========================================");
      print("DELETE MANY CONVERSATIONS");
      print("COUNT: ${conversationIds.length}");

      await _conversationService.deleteManyConversations(conversationIds);

      _conversations.removeWhere((e) => conversationIds.contains(e.id));

      if (_selectedConversation != null &&
          conversationIds.contains(_selectedConversation!.id)) {
        _selectedConversation = _conversations.isNotEmpty
            ? _conversations.first
            : null;
      }

      notifyListeners();

      print("DELETE MANY CONVERSATIONS SUCCESS");
      print("========================================");
      return true;
    } catch (e) {
      print("DELETE MANY CONVERSATIONS ERROR: $e");

      if (e is UnauthorizedException) {
        await AuthGuard.handleUnauthorized();
        return false;
      }

      _errorMessage = "Gagal menghapus percakapan";

      notifyListeners();
      return false;
    } finally {
      _setDeleting(false);
    }
  }

  // =========================================================
  // DELETE ALL CONVERSATIONS
  // =========================================================

  Future<bool> deleteAllConversations() async {
    try {
      _setDeleting(true);

      _errorMessage = null;

      print("========================================");
      print("DELETE ALL CONVERSATIONS");

      final deletedCount = await _conversationService.deleteAllConversations();

      _conversations = [];
      _selectedConversation = null;

      notifyListeners();

      print("DELETE ALL CONVERSATIONS SUCCESS: $deletedCount item");
      print("========================================");
      return true;
    } catch (e) {
      print("DELETE ALL CONVERSATIONS ERROR: $e");

      if (e is UnauthorizedException) {
        await AuthGuard.handleUnauthorized();
        return false;
      }

      _errorMessage = "Gagal menghapus semua percakapan";

      notifyListeners();
      return false;
    } finally {
      _setDeleting(false);
    }
  }

  // =========================================================
  // REMOVE LOCAL CONVERSATION
  // =========================================================

  void removeConversation(String conversationId) {
    _conversations.removeWhere((e) => e.id == conversationId);

    // =====================================================
    // CLEAR SELECTED
    // =====================================================

    if (_selectedConversation?.id == conversationId) {
      _selectedConversation = null;
    }

    notifyListeners();
  }

  // =========================================================
  // CLEAR
  // =========================================================

  void clear() {
    _conversations = [];

    _selectedConversation = null;

    _errorMessage = null;

    notifyListeners();
  }
}
