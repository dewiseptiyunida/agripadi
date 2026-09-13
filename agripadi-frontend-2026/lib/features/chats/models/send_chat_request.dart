import 'attachment_request.dart';

class SendChatRequest {
  final String? conversationId;
  final String? message;
  final List<ChatAttachment> attachments;
  final List<DetectionCandidate> detections;

  const SendChatRequest({
    this.conversationId,
    this.message,
    this.attachments = const [],
    this.detections = const [],
  });

  bool get hasImage => attachments.any((item) => item.type == "image");

  String get content {
    final text = message?.trim() ?? "";

    if (text.isNotEmpty) {
      return text;
    }

    return hasImage ? "Mohon analisis gambar hama ini." : "";
  }

  Map<String, dynamic> toJson() {
    return {
      if (conversationId != null && conversationId!.isNotEmpty)
        "conversation_id": conversationId,
      "type": hasImage ? "image" : "text",
      "content": content,
      if (attachments.isNotEmpty)
        "attachments": attachments.map((item) => item.toJson()).toList(),
      if (detections.isNotEmpty)
        "detections": detections.map((item) => item.toJson()).toList(),
    };
  }
}

class DetectionCandidate {
  final String label;
  final double confidence;
  final String model;
  final String? source;
  final int? rank;
  final int? latencyMs;

  const DetectionCandidate({
    required this.label,
    required this.confidence,
    required this.model,
    this.source,
    this.rank,
    this.latencyMs,
  });

  factory DetectionCandidate.fromJson(Map<String, dynamic> json) {
    return DetectionCandidate(
      label: json["label"]?.toString() ?? "",
      confidence: _asDouble(json["confidence"]),
      model: json["model"]?.toString() ?? "",
      source: json["source"]?.toString(),
      rank: _nullableInt(json["rank"]),
      latencyMs: _nullableInt(json["latency_ms"] ?? json["latencyMs"]),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      "label": label,
      "confidence": confidence,
      "model": model,
      if (source != null && source!.isNotEmpty) "source": source,
      if (rank != null) "rank": rank,
      if (latencyMs != null) "latency_ms": latencyMs,
    };
  }

  static double _asDouble(dynamic value) {
    if (value is num) {
      return value.toDouble();
    }

    return double.tryParse(value?.toString() ?? "") ?? 0;
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
