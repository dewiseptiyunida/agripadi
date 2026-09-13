import 'dart:math' as math;

import 'package:agripadi/features/chats/models/send_chat_request.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:image/image.dart' as img;
import 'package:image_picker/image_picker.dart';
import 'package:tflite_flutter/tflite_flutter.dart';

class OnDeviceCnnService {
  OnDeviceCnnService._();

  static final OnDeviceCnnService instance = OnDeviceCnnService._();

  static const String _modelAsset =
      'assets/model/mobilenetv3_small_fp16.tflite';
  static const String _labelAsset = 'assets/model/label.txt';
  static const int _defaultInputSize = 224;
  static const int _topK = 3;

  Interpreter? _interpreter;
  List<String>? _labels;

  /// Memuat interpreter dan label lebih awal agar scan pertama lebih responsif.
  Future<void> preload() => _ensureLoaded();

  Future<List<DetectionCandidate>> classifyImage(XFile imageFile) async {
    await _ensureLoaded();

    final interpreter = _interpreter;
    final labels = _labels ?? const <String>[];

    if (interpreter == null || labels.isEmpty) {
      throw StateError('Model CNN on-device belum berhasil dimuat.');
    }

    final imageBytes = await imageFile.readAsBytes();
    if (imageBytes.isEmpty) {
      throw ArgumentError('File gambar kosong atau tidak dapat dibaca.');
    }

    final decodedImage = img.decodeImage(imageBytes);
    if (decodedImage == null) {
      throw ArgumentError('File gambar tidak dapat dibaca oleh model CNN.');
    }

    // Kamera ponsel sering menyimpan orientasi pada metadata EXIF. Baking
    // diperlukan agar gambar yang masuk ke model tidak terputar 90/180 derajat.
    final orientedImage = img.bakeOrientation(decodedImage);

    final inputTensor = interpreter.getInputTensor(0);
    final outputTensor = interpreter.getOutputTensor(0);
    final inputShape = inputTensor.shape;
    final outputShape = outputTensor.shape;

    if (inputShape.length != 4 ||
        inputShape.first != 1 ||
        inputShape.last != 3) {
      throw StateError(
        'Format input model tidak didukung: $inputShape. '
        'Model harus menerima tensor [1, tinggi, lebar, 3].',
      );
    }

    final inputHeight = inputShape.length >= 3
        ? inputShape[inputShape.length - 3]
        : _defaultInputSize;
    final inputWidth = inputShape.length >= 2
        ? inputShape[inputShape.length - 2]
        : _defaultInputSize;

    if (inputHeight <= 0 || inputWidth <= 0) {
      throw StateError('Ukuran input model tidak valid: $inputShape.');
    }

    final classCount = outputShape.isNotEmpty ? outputShape.last : labels.length;
    if (classCount <= 0) {
      throw StateError('Jumlah kelas keluaran model tidak valid: $outputShape.');
    }
    if (classCount != labels.length) {
      throw StateError(
        'Jumlah label (${labels.length}) tidak sama dengan keluaran model '
        '($classCount). Periksa assets/model/label.txt.',
      );
    }

    final input = _buildInputTensor(
      orientedImage,
      inputHeight: inputHeight,
      inputWidth: inputWidth,
    );
    final output = List.generate(
      1,
      (_) => List<double>.filled(classCount, 0),
      growable: false,
    );

    // Catat waktu inferensi murni, bukan waktu memuat model atau membaca gambar.
    final inferenceStopwatch = Stopwatch()..start();
    interpreter.run(input, output);
    inferenceStopwatch.stop();

    if (output.first.any((value) => !value.isFinite)) {
      throw StateError('Model CNN menghasilkan nilai keluaran yang tidak valid.');
    }

    final probabilities = _normalizeOutput(output.first);
    final rankedIndexes = List<int>.generate(
      probabilities.length,
      (index) => index,
    )..sort((a, b) => probabilities[b].compareTo(probabilities[a]));

    final results = <DetectionCandidate>[];
    for (final index
        in rankedIndexes.take(math.min(_topK, rankedIndexes.length))) {
      results.add(
        DetectionCandidate(
          label: _normalizeLabel(labels[index]),
          confidence: probabilities[index].clamp(0, 1).toDouble(),
          model: 'mobilenetv3_small_fp16',
          source: 'on_device_flutter',
          rank: results.length + 1,
          latencyMs: inferenceStopwatch.elapsedMilliseconds,
        ),
      );
    }

    if (results.isEmpty) {
      throw StateError('Model CNN tidak mengembalikan kandidat deteksi.');
    }

    debugPrint(
      'ON DEVICE CNN RESULT: ${results.map((item) => item.toJson()).toList()}',
    );

    return results;
  }

  Future<void> _ensureLoaded() async {
    if (_interpreter != null && _labels != null) {
      return;
    }

    final labels = await _loadLabels();
    if (labels.isEmpty) {
      throw StateError('File label CNN kosong.');
    }

    final options = InterpreterOptions();
    if (defaultTargetPlatform == TargetPlatform.android) {
      options.threads = 4;
    }

    final interpreter = await Interpreter.fromAsset(
      _modelAsset,
      options: options,
    );

    // Simpan setelah seluruh proses berhasil agar pemanggilan berikutnya tidak
    // melihat state setengah terinisialisasi.
    _labels = labels;
    _interpreter = interpreter;
  }

  Future<List<String>> _loadLabels() async {
    final raw = await rootBundle.loadString(_labelAsset);

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

    return List.generate(
      1,
      (_) => List.generate(
        inputHeight,
        (y) => List.generate(
          inputWidth,
          (x) {
            final pixel = resized.getPixel(x, y);
            return <double>[
              _normalizePixel(pixel.r, 0),
              _normalizePixel(pixel.g, 1),
              _normalizePixel(pixel.b, 2),
            ];
          },
          growable: false,
        ),
        growable: false,
      ),
      growable: false,
    );
  }

  double _normalizePixel(num channel, int channelIndex) {
    const mean = [0.485, 0.456, 0.406];
    const std = [0.229, 0.224, 0.225];

    final value = channel.toDouble() / 255.0;
    return (value - mean[channelIndex]) / std[channelIndex];
  }

  List<double> _normalizeOutput(List<double> raw) {
    if (raw.isEmpty) {
      return raw;
    }
    if (raw.any((value) => !value.isFinite)) {
      throw StateError('Keluaran model mengandung NaN atau Infinity.');
    }

    final allProbability = raw.every((value) => value >= 0 && value <= 1);
    final sum = raw.fold<double>(0, (previous, value) => previous + value);

    if (allProbability && sum > 0.95 && sum < 1.05) {
      return List<double>.unmodifiable(raw);
    }

    final maxValue = raw.reduce(math.max);
    final expValues = raw
        .map((value) => math.exp(value - maxValue))
        .toList(growable: false);
    final expSum = expValues.fold<double>(
      0,
      (previous, value) => previous + value,
    );

    if (!expSum.isFinite || expSum <= 0) {
      throw StateError('Keluaran model tidak dapat dinormalisasi.');
    }

    return expValues
        .map((value) => value / expSum)
        .toList(growable: false);
  }

  String _normalizeLabel(String label) {
    final normalized = label
        .trim()
        .toLowerCase()
        .replaceAll('_', ' ')
        .replaceAll('-', ' ')
        .replaceAll(RegExp(r'\s+'), ' ');

    switch (normalized) {
      case 'brown planthopper':
      case 'wereng cokelat':
      case 'wereng batang coklat':
      case 'wereng batang cokelat':
        return 'wereng batang cokelat';
      case 'rice bug':
      case 'leptocorisa oratorius':
      case 'walang sangit':
        return 'walang sangit';
      case 'stem borer':
      case 'yellow stem borer':
      case 'penggerek batang padi':
      case 'penggerek batang':
        return 'penggerek batang';
      case 'healthy':
      case 'normal':
      case 'no pest':
      case 'none':
      case 'tanpa hama':
      case 'tidak ada hama':
        return 'tidak ada hama';
      default:
        return normalized;
    }
  }

  void close() {
    _interpreter?.close();
    _interpreter = null;
    _labels = null;
  }
}
