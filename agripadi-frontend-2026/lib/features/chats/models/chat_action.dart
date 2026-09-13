class ChatAction {
  final String type;
  final String label;
  final String value;

  const ChatAction({
    required this.type,
    required this.label,
    required this.value,
  });

  bool get isSelectable {
    return type == 'select_symptom' ||
        type == 'select_severity' ||
        type == 'select_growth_stage';
  }

  bool get isUploadImage => type == 'upload_image';

  bool get isContinueConsultation => type == 'continue_consultation';

  factory ChatAction.fromJson(Map<String, dynamic> json) {
    return ChatAction(
      type: json['type']?.toString() ?? '',
      label: json['label']?.toString() ?? '',
      value: json['value']?.toString() ?? '',
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'type': type,
      'label': label,
      'value': value,
    };
  }
}
