class ChatAttachment {
  final String type;
  final String url;
  final String mimeType;
  final int size;
  final int? width;
  final int? height;
  final String? thumbnailUrl;
  final String? checksum;

  const ChatAttachment({
    required this.type,
    required this.url,
    required this.mimeType,
    required this.size,
    this.width,
    this.height,
    this.thumbnailUrl,
    this.checksum,
  });

  factory ChatAttachment.fromJson(Map<String, dynamic> json) {
    return ChatAttachment(
      type: json["type"]?.toString() ?? "image",
      url: (json["url"] ?? json["file_url"] ?? "").toString(),
      mimeType: (json["mime_type"] ?? json["mimeType"] ?? "").toString(),
      size: _asInt(json["size"] ?? json["file_size"]),
      width: _nullableInt(json["width"]),
      height: _nullableInt(json["height"]),
      thumbnailUrl: json["thumbnail_url"]?.toString(),
      checksum: json["checksum"]?.toString(),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      "type": type,
      "url": url,
      "mime_type": mimeType,
      "size": size,
      if (width != null) "width": width,
      if (height != null) "height": height,
      if (thumbnailUrl != null) "thumbnail_url": thumbnailUrl,
      if (checksum != null) "checksum": checksum,
    };
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

  static int? _nullableInt(dynamic value) {
    if (value == null) {
      return null;
    }

    return _asInt(value);
  }
}
