class RecommendationData {
  final String? pestName;
  final String? severityName;
  final String? growthStageName;
  final double? cnnConfidence;
  final double? finalConfidence;
  final List<String> pesticides;
  final String? education;
  final List<String> reasoning;
  final List<String> prevention;
  final List<String> warnings;

  const RecommendationData({
    this.pestName,
    this.severityName,
    this.growthStageName,
    this.cnnConfidence,
    this.finalConfidence,
    this.pesticides = const [],
    this.education,
    this.reasoning = const [],
    this.prevention = const [],
    this.warnings = const [],
  });

  bool get isEmpty {
    return (pestName == null || pestName!.trim().isEmpty) &&
        (severityName == null || severityName!.trim().isEmpty) &&
        (growthStageName == null || growthStageName!.trim().isEmpty) &&
        pesticides.isEmpty &&
        (education == null || education!.trim().isEmpty) &&
        reasoning.isEmpty &&
        prevention.isEmpty &&
        warnings.isEmpty;
  }

  factory RecommendationData.fromDiagnosis(Map<String, dynamic> json) {
    final recommendations = _asList(json["recommendations"]);
    final llm = _asMap(json["llm"]);
    final deterministic = _asMap(json["deterministic"]);
    final confidence = _asMap(json["confidence"]);
    final pest = _asMap(json["pest"]);
    final severity = _asMap(json["severity"]);
    final growthStage = _asMap(json["growth_stage"]);

    final pesticideTexts = recommendations
        .map((item) {
          final map = _asMap(item);
          final displayName = _firstText([
            map["display_name"],
            map["product_name"],
            map["pesticide_type"],
          ]);
          final dosage = _formatDosage(_asMap(map["dosage"]));
          final timingMap = _asMap(map["application_timing"]);
          final timing = timingMap["timing_window"]?.toString() ?? "";
          final timingInstruction =
              timingMap["timing_instruction"]?.toString() ?? "";
          final ingredients = _asList(map["ingredients"])
              .map((ingredient) {
                final ingredientMap = _asMap(ingredient);
                final name = ingredientMap["name"]?.toString() ?? "";
                final concentration =
                    ingredientMap["concentration_raw"]?.toString() ?? "";
                return [name, concentration]
                    .where((value) => value.trim().isNotEmpty)
                    .join(" ");
              })
              .where((value) => value.trim().isNotEmpty)
              .join(", ");

          final details = <String>[
            if (ingredients.trim().isNotEmpty)
              "Bahan aktif: ${_compactText(ingredients, maxChars: 90)}",
            if (dosage.trim().isNotEmpty)
              "Dosis: ${_compactText(dosage, maxChars: 70)}",
            if (timing.trim().isNotEmpty)
              "Waktu: ${_compactText(timing, maxChars: 90)}",
            if (timing.trim().isEmpty && timingInstruction.trim().isNotEmpty)
              "Waktu: ${_compactText(timingInstruction, maxChars: 90)}",
          ];

          if (displayName.trim().isEmpty && details.isEmpty) {
            return "";
          }

          if (details.isEmpty) {
            return displayName;
          }

          return "$displayName\n${details.join("\n")}";
        })
        .where((item) => item.trim().isNotEmpty)
        .toList();

    final mixingWarnings = recommendations
        .expand(
          (item) => _asList(_asMap(_asMap(item)["safety"])["mixing_warning"]),
        )
        .map((item) => item.toString())
        .where((item) => item.trim().isNotEmpty)
        .toList();

    final safetyWarnings = recommendations
        .map((item) => _asMap(_asMap(item)["safety"]))
        .expand((safety) {
          return [
            if (safety["toxicity_class"] != null)
              "Kelas toksisitas: ${safety["toxicity_class"]}",
            if (safety["reentry_interval"] != null)
              "Interval masuk kembali: ${safety["reentry_interval"]}",
            if (safety["pre_harvest_interval"] != null)
              "Interval pra-panen: ${safety["pre_harvest_interval"]}",
            if (safety["max_application"] != null)
              "Maksimal aplikasi: ${safety["max_application"]}",
          ];
        })
        .where((item) => item.trim().isNotEmpty)
        .toList();

    final llmWarnings = _asList(
      llm["warnings"],
    ).map((item) => item.toString()).where((item) => item.trim().isNotEmpty);

    final education = _compactText(
      (llm["summary"] ?? deterministic["summary"])?.toString() ?? "",
      maxChars: 220,
      maxSentences: 2,
    );

    return RecommendationData(
      pestName: _firstText([pest["name"], pest["label_name"]]),
      severityName: severity["name"]?.toString(),
      growthStageName: growthStage["name"]?.toString(),
      cnnConfidence: _nullableDouble(confidence["cnn"]),
      finalConfidence: _nullableDouble(confidence["final_score"]),
      pesticides: pesticideTexts.take(2).toList(),
      education: education,
      reasoning: _asList(deterministic["reasoning"])
          .map((item) => _compactText(item.toString(), maxChars: 120))
          .where((item) => item.trim().isNotEmpty)
          .take(2)
          .toList(),
      prevention: _asList(llm["prevention"])
          .map((item) => _compactText(item.toString(), maxChars: 120))
          .where((item) => item.trim().isNotEmpty)
          .take(2)
          .toList(),
      warnings: [...safetyWarnings, ...mixingWarnings, ...llmWarnings]
          .map((item) => _compactText(item.toString(), maxChars: 120))
          .where((item) => item.trim().isNotEmpty)
          .take(3)
          .toList(),
    );
  }

  factory RecommendationData.fromOrchestrator(Map<String, dynamic> json) {
    final payload = _asMap(json["payload"]);
    final diagnose = _asMap(payload["diagnose"]);
    final result = _asMap(diagnose["result"]);

    if (result.isEmpty) {
      return const RecommendationData();
    }

    return RecommendationData.fromDiagnosis(result);
  }

  factory RecommendationData.fromMessageJson(Map<String, dynamic> json) {
    final fromPayload = RecommendationData.fromOrchestrator(json);
    if (!fromPayload.isEmpty) {
      return fromPayload;
    }

    final recommendation = _asMap(json["recommendation"]);
    if (recommendation.isNotEmpty) {
      return RecommendationData.fromDiagnosis(recommendation);
    }

    final diagnose = _asMap(json["diagnose"]);
    final result = _asMap(diagnose["result"]);
    if (result.isNotEmpty) {
      return RecommendationData.fromDiagnosis(result);
    }

    return const RecommendationData();
  }

  Map<String, dynamic> toPayloadJson() {
    return {
      "diagnose": {
        "result": {
          "pest": {"name": pestName, "label_name": pestName},
          "severity": {"name": severityName},
          "growth_stage": {"name": growthStageName},
          "confidence": {
            "cnn": cnnConfidence,
            "final_score": finalConfidence,
          },
          "recommendations": pesticides
              .map((item) => {"display_name": item})
              .toList(),
          "deterministic": {
            "summary": education,
            "reasoning": reasoning,
          },
          "llm": {
            "summary": education,
            "prevention": prevention,
            "warnings": warnings,
          },
        },
      },
    };
  }

  static Map<String, dynamic> _asMap(dynamic value) {
    if (value is Map<String, dynamic>) {
      return value;
    }

    if (value is Map) {
      return Map<String, dynamic>.from(value);
    }

    return <String, dynamic>{};
  }

  static List<dynamic> _asList(dynamic value) {
    if (value is List) {
      return value;
    }

    return const [];
  }

  static String _firstText(List<dynamic> values) {
    for (final value in values) {
      final text = value?.toString().trim() ?? "";
      if (text.isNotEmpty) {
        return text;
      }
    }
    return "";
  }

  static String _formatDosage(Map<String, dynamic> dosage) {
    final raw = _firstText([
      dosage["dose_raw"],
      dosage["raw"],
      dosage["dose"],
    ]);
    final unit = _firstText([
      dosage["dose_unit"],
      dosage["unit"],
      dosage["satuan"],
    ]);

    if (raw.isEmpty) {
      return "";
    }

    if (unit.isEmpty || unit.toLowerCase() == "label") {
      return raw;
    }

    final lowerRaw = raw.toLowerCase();
    final lowerUnit = unit.toLowerCase();
    if (lowerRaw.contains(lowerUnit)) {
      return raw;
    }

    final severityRangePattern = RegExp(
      r'^([^;]+);\s*(batas bawah|nilai tengah|batas atas)\s+rentang label\s+(.+)$',
      caseSensitive: false,
    );
    final severityRangeMatch = severityRangePattern.firstMatch(raw);
    if (severityRangeMatch != null) {
      final selectedValue = (severityRangeMatch.group(1) ?? '').trim();
      final selectionLabel = (severityRangeMatch.group(2) ?? '').trim();
      final rangeValue = (severityRangeMatch.group(3) ?? '').trim();
      return '$selectedValue $unit ($selectionLabel dari rentang label '
              '$rangeValue $unit)'
          .replaceAll(RegExp(r'\s+'), ' ')
          .trim();
    }

    var text = raw;
    final rangePattern = RegExp(
      r'\((rentang label:\s*)([^)]+)\)',
      caseSensitive: false,
    );
    text = text.replaceAllMapped(rangePattern, (match) {
      final prefix = match.group(1) ?? "";
      final rangeValue = (match.group(2) ?? "").trim();
      if (rangeValue.toLowerCase().contains(lowerUnit)) {
        return match.group(0) ?? "";
      }
      return "($prefix$rangeValue $unit)";
    });

    final parenthesisIndex = text.indexOf("(");
    if (parenthesisIndex > 0) {
      final before = text.substring(0, parenthesisIndex).trimRight();
      final after = text.substring(parenthesisIndex).trimLeft();
      if (before.isNotEmpty) {
        return "$before $unit $after".replaceAll(RegExp(r'\s+'), ' ').trim();
      }
    }

    return "$text $unit".replaceAll(RegExp(r'\s+'), ' ').trim();
  }

  static String _compactText(
    String value, {
    int maxChars = 160,
    int maxSentences = 1,
  }) {
    var text = value.trim().replaceAll(RegExp(r'\s+'), ' ');
    if (text.isEmpty) {
      return '';
    }

    if (maxSentences > 0) {
      final matches = RegExp(r'[^.!?]+[.!?](?=\s|$)').allMatches(text).toList();
      if (matches.isNotEmpty) {
        text = matches
            .take(maxSentences)
            .map((match) => match.group(0)!.trim())
            .join(' ');
      }
    }

    if (text.length > maxChars) {
      text = text.substring(0, maxChars).trimRight();
      text = text.replaceAll(RegExp(r'[,;:\-\s]+$'), '');
      text = '$text...';
    }

    return text;
  }

  static double? _nullableDouble(dynamic value) {
    if (value == null) {
      return null;
    }
    if (value is num) {
      return value.toDouble();
    }
    return double.tryParse(value.toString());
  }
}
