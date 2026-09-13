import 'dart:convert';
import 'dart:io';
import 'dart:math' as math;

import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:image/image.dart' as img;
import 'package:tflite_flutter/tflite_flutter.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  const casesPath = String.fromEnvironment('CNN_V2_CASES');
  const outputPath = String.fromEnvironment('CNN_V2_OUTPUT');
  const hasEvaluationInput = casesPath != '' && outputPath != '';

  test(
    'Evaluasi TensorFlow Lite pada Flutter',
    () async {
      final cases = _readCases(File(casesPath));
      expect(cases, isNotEmpty, reason: 'CSV kasus uji kosong atau path salah.');

      final labels = await _loadLabels('assets/model/label.txt');

      final options = InterpreterOptions();
      options.threads = 4;

      final interpreter = await Interpreter.fromAsset(
        'assets/model/mobilenetv3_small_fp16.tflite',
        options: options,
      );

      try {
        final inputTensor = interpreter.getInputTensor(0);
        final outputTensor = interpreter.getOutputTensor(0);

        final inputShape = inputTensor.shape;
        final outputShape = outputTensor.shape;

        final inputHeight = inputShape.length >= 3
            ? inputShape[inputShape.length - 3]
            : 224;
        final inputWidth = inputShape.length >= 2
            ? inputShape[inputShape.length - 2]
            : 224;
        final classCount = outputShape.isNotEmpty
            ? outputShape.last
            : labels.length;

        final outFile = File(outputPath);
        await outFile.parent.create(recursive: true);
        final sink = outFile.openWrite(mode: FileMode.write);

        var correct = 0;
        var totalLatency = 0;
        final latencies = <int>[];

        for (var i = 0; i < cases.length; i++) {
          final item = cases[i];

          final imageFile = File(item.imagePath);
          if (!imageFile.existsSync()) {
            throw StateError('File gambar tidak ditemukan: ${item.imagePath}');
          }

          final bytes = await imageFile.readAsBytes();
          final decoded = img.decodeImage(bytes);
          if (decoded == null) {
            throw StateError('Gambar tidak dapat dibaca: ${item.imagePath}');
          }

          final input = _buildInputTensor(
            img.bakeOrientation(decoded),
            inputHeight: inputHeight,
            inputWidth: inputWidth,
          );

          final output = List<List<double>>.generate(
            1,
            (_) => List<double>.filled(classCount, 0.0),
            growable: false,
          );

          final stopwatch = Stopwatch()..start();
          interpreter.run(input, output);
          stopwatch.stop();

          final probs = _normalizeOutput(output.first);
          final ranked = List<int>.generate(probs.length, (index) => index)
            ..sort((a, b) => probs[b].compareTo(probs[a]));

          final top3 = <Map<String, dynamic>>[];
          for (final index in ranked.take(math.min(3, ranked.length))) {
            if (index >= labels.length) continue;
            top3.add({
              'label': _normalizeLabel(labels[index]),
              'confidence': probs[index],
            });
          }

          final predictedLabel = top3.isEmpty
              ? ''
              : top3.first['label'] as String;
          final trueLabel = _normalizeLabel(item.trueLabel);
          final isCorrect = predictedLabel == trueLabel;

          if (isCorrect) correct++;

          final latencyMs = stopwatch.elapsedMilliseconds;
          totalLatency += latencyMs;
          latencies.add(latencyMs);

          final row = {
            'no': i + 1,
            'image_path': item.imagePath,
            'true_label': trueLabel,
            'label': predictedLabel,
            'confidence': top3.isEmpty ? 0.0 : top3.first['confidence'],
            'is_correct': isCorrect,
            'model': 'mobilenetv3_small_fp16',
            'source': 'on_device_flutter',
            'rank': 1,
            'latency_ms': latencyMs,
            'top3': top3,
          };

          sink.writeln(jsonEncode(row));
        }

        await sink.flush();
        await sink.close();

        latencies.sort();
        final summary = {
          'total_images': cases.length,
          'correct': correct,
          'wrong': cases.length - correct,
          'accuracy': cases.isEmpty ? 0.0 : correct / cases.length,
          'latency_ms_mean': cases.isEmpty ? 0.0 : totalLatency / cases.length,
          'latency_ms_min': latencies.isEmpty ? 0 : latencies.first,
          'latency_ms_median': latencies.isEmpty
              ? 0
              : latencies[latencies.length ~/ 2],
          'latency_ms_max': latencies.isEmpty ? 0 : latencies.last,
          'model': 'mobilenetv3_small_fp16',
          'source': 'on_device_flutter',
          'note':
            'Preprocessing menggunakan ImageNet mean/std sesuai export_summary.json.',
        };

        await File('$outputPath.summary.json').writeAsString(
          const JsonEncoder.withIndent('  ').convert(summary),
        );

        expect(cases.length, greaterThan(0));
      } finally {
        interpreter.close();
      }
    },
    skip: hasEvaluationInput
        ? false
        : 'Gunakan --dart-define=CNN_V2_CASES=... dan '
              '--dart-define=CNN_V2_OUTPUT=... untuk menjalankan evaluasi dataset.',
  );
}

class _CaseItem {
  final String imagePath;
  final String trueLabel;

  const _CaseItem(this.imagePath, this.trueLabel);
}

List<_CaseItem> _readCases(File file) {
  if (!file.existsSync()) {
    throw StateError('CSV kasus uji tidak ditemukan: ${file.path}');
  }

  final lines = file.readAsLinesSync().where((line) => line.trim().isNotEmpty).toList();
  if (lines.isEmpty) return const [];

  final result = <_CaseItem>[];
  final start = lines.first.toLowerCase().contains('image_path') ? 1 : 0;

  for (var i = start; i < lines.length; i++) {
    final parts = _splitCsvLine(lines[i]);
    if (parts.length < 2) continue;
    result.add(_CaseItem(parts[0].trim(), parts[1].trim()));
  }

  return result;
}

List<String> _splitCsvLine(String line) {
  final result = <String>[];
  final buffer = StringBuffer();
  var inQuotes = false;

  for (var i = 0; i < line.length; i++) {
    final char = line[i];

    if (char == '"') {
      inQuotes = !inQuotes;
    } else if (char == ',' && !inQuotes) {
      result.add(buffer.toString());
      buffer.clear();
    } else {
      buffer.write(char);
    }
  }

  result.add(buffer.toString());
  return result;
}

Future<List<String>> _loadLabels(String assetPath) async {
  final raw = await rootBundle.loadString(assetPath);
  return raw
      .split(RegExp(r'\r?\n'))
      .map((line) => line.trim())
      .where((line) => line.isNotEmpty)
      .toList(growable: false);
}

Object _buildInputTensor(
  img.Image source, {
  required int inputHeight,
  required int inputWidth,
}) {
  final resized = img.copyResize(
    source,
    width: inputWidth,
    height: inputHeight,
    interpolation: img.Interpolation.linear,
  );

  const mean = [0.485, 0.456, 0.406];
  const std = [0.229, 0.224, 0.225];

  return List.generate(
    1,
    (_) => List.generate(
      inputHeight,
      (y) => List.generate(
        inputWidth,
        (x) {
          final pixel = resized.getPixel(x, y);

          final r = (pixel.r.toDouble() / 255.0 - mean[0]) / std[0];
          final g = (pixel.g.toDouble() / 255.0 - mean[1]) / std[1];
          final b = (pixel.b.toDouble() / 255.0 - mean[2]) / std[2];

          return <double>[r, g, b];
        },
        growable: false,
      ),
      growable: false,
    ),
    growable: false,
  );
}

List<double> _normalizeOutput(List<double> raw) {
  if (raw.isEmpty) return raw;

  final allProbability = raw.every((value) => value >= 0 && value <= 1);
  final sum = raw.fold<double>(0.0, (prev, value) => prev + value);

  if (allProbability && sum > 0.95 && sum < 1.05) {
    return raw;
  }

  final maxValue = raw.reduce((a, b) => a > b ? a : b);
  final expValues = raw.map((value) => math.exp(value - maxValue)).toList(growable: false);
  final expSum = expValues.fold<double>(0.0, (prev, value) => prev + value);

  if (expSum <= 0) {
    return raw.map((_) => 0.0).toList(growable: false);
  }

  return expValues.map((value) => value / expSum).toList(growable: false);
}

String _normalizeLabel(String label) {
  final normalized = label
      .trim()
      .toLowerCase()
      .replaceAll('_', ' ')
      .replaceAll('-', ' ')
      .replaceAll(RegExp(r'\s+'), ' ');

  switch (normalized) {
    case 'wereng cokelat':
    case 'wereng batang coklat':
    case 'wereng batang cokelat':
      return 'wereng batang cokelat';
    case 'walang sangit':
      return 'walang sangit';
    case 'penggerek batang padi':
    case 'penggerek batang':
      return 'penggerek batang';
    case 'tidak ada hama':
    case 'tanpa hama':
      return 'tidak ada hama';
    default:
      return normalized;
  }
}
