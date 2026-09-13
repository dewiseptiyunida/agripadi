import 'dart:async';
import 'dart:convert';

import 'package:agripadi/core/storage/secure_storage.dart';
import 'package:flutter/foundation.dart';
import 'package:http/http.dart' as http;
import 'package:image_picker/image_picker.dart';

class ApiConfig {
  static const String host = "https://agripadibackendv2.petanitech.com";

  static const String inferenceMode = "on_device_tflite";
  static const String inferenceModeLabel = "On-device TFLite";
  static const String inferenceModeDescription = "CNN MobileNetV3-Small berjalan langsung di Flutter menggunakan TensorFlow Lite";

  static const String api = "$host/api";
}

class ApiClient {
  ApiClient._();

  static final ApiClient instance = ApiClient._();

  // =========================================
  // BASE URL
  // =========================================

  static const String baseUrl = ApiConfig.api;
  static const String serverUrl = ApiConfig.host;
  static const Duration requestTimeout = Duration(seconds: 75);
  static const Duration uploadTimeout = Duration(seconds: 90);

  // =========================================
  // DEFAULT HEADERS
  // =========================================

  Future<Map<String, String>> _headers({bool withAuth = false}) async {
    final headers = {
      "Content-Type": "application/json",
      "Accept": "application/json",
    };

    if (withAuth) {
      final token = (await TokenStorage.getAccessToken())?.trim();

      if (token == null || token.isEmpty) {
        throw const UnauthorizedException(
          "Sesi login tidak ditemukan. Silakan masuk kembali.",
        );
      }

      headers["Authorization"] = "Bearer $token";
    }

    return headers;
  }

  // =========================================
  // GET
  // =========================================

  Future<dynamic> get(String endpoint, {bool withAuth = false}) async {
    try {
      final response = await http
          .get(
            Uri.parse("$baseUrl$endpoint"),
            headers: await _headers(withAuth: withAuth),
          )
          .timeout(requestTimeout);

      return _processResponse(response);
    } on TimeoutException {
      throw ApiException(
        "Koneksi ke server timeout. Periksa koneksi internet Anda dan coba lagi.",
      );
    } on http.ClientException {
      throw ApiException(
        "Tidak dapat terhubung ke server. Periksa koneksi internet Anda.",
      );
    } on FormatException {
      throw const ApiException("Respons server tidak valid.");
    } catch (e) {
      rethrow;
    }
  }

  // =========================================
  // POST
  // =========================================

  Future<dynamic> post(
    String endpoint, {
    Map<String, dynamic>? body,
    bool withAuth = false,
  }) async {
    try {
      final response = await http
          .post(
            Uri.parse("$baseUrl$endpoint"),
            headers: await _headers(withAuth: withAuth),
            body: jsonEncode(body ?? {}),
          )
          .timeout(requestTimeout);

      return _processResponse(response);
    } on TimeoutException {
      throw ApiException(
        "Request timeout. Server tidak merespons tepat waktu.",
      );
    } on http.ClientException {
      throw ApiException(
        "Tidak dapat terhubung ke server. Periksa koneksi internet Anda.",
      );
    } on FormatException {
      throw const ApiException("Respons server tidak valid.");
    } catch (e) {
      rethrow;
    }
  }

  // =========================================
  // PUT
  // =========================================

  Future<dynamic> put(
    String endpoint, {
    Map<String, dynamic>? body,
    bool withAuth = false,
  }) async {
    try {
      final response = await http
          .put(
            Uri.parse("$baseUrl$endpoint"),
            headers: await _headers(withAuth: withAuth),
            body: jsonEncode(body ?? {}),
          )
          .timeout(requestTimeout);

      return _processResponse(response);
    } on TimeoutException {
      throw ApiException(
        "Request timeout. Server tidak merespons tepat waktu.",
      );
    } on http.ClientException {
      throw ApiException(
        "Tidak dapat terhubung ke server. Pastikan aplikasi berjalan dan terhubung ke internet.",
      );
    } on FormatException {
      throw const ApiException("Respons server tidak valid.");
    } catch (e) {
      rethrow;
    }
  }

  // =========================================
  // PATCH
  // =========================================

  Future<dynamic> patch(
    String endpoint, {
    Map<String, dynamic>? body,
    bool withAuth = false,
  }) async {
    try {
      final response = await http
          .patch(
            Uri.parse("$baseUrl$endpoint"),
            headers: await _headers(withAuth: withAuth),
            body: jsonEncode(body ?? {}),
          )
          .timeout(requestTimeout);

      return _processResponse(response);
    } on TimeoutException {
      throw ApiException(
        "Request timeout. Server tidak merespons tepat waktu.",
      );
    } on http.ClientException {
      throw ApiException(
        "Tidak dapat terhubung ke server. Pastikan aplikasi berjalan dan terhubung ke internet.",
      );
    } on FormatException {
      throw const ApiException("Respons server tidak valid.");
    } catch (e) {
      rethrow;
    }
  }

  // =========================================
  // DELETE
  // =========================================

  Future<dynamic> delete(
    String endpoint, {
    Map<String, dynamic>? body,
    bool withAuth = false,
  }) async {
    try {
      final response = await http
          .delete(
            Uri.parse("$baseUrl$endpoint"),
            headers: await _headers(withAuth: withAuth),
            body: body != null ? jsonEncode(body) : null,
          )
          .timeout(requestTimeout);

      return _processResponse(response);
    } on TimeoutException {
      throw ApiException(
        "Request timeout. Server tidak merespons tepat waktu.",
      );
    } on http.ClientException {
      throw ApiException(
        "Tidak dapat terhubung ke server. Pastikan aplikasi berjalan dan terhubung ke internet.",
      );
    } on FormatException {
      throw const ApiException("Respons server tidak valid.");
    } catch (e) {
      rethrow;
    }
  }

  // =========================================
  // MULTIPART IMAGE UPLOAD
  // =========================================

  Future<dynamic> uploadImage({
    required XFile file,
    bool skipServerCnn = true,
    bool withAuth = true,
  }) async {
    try {
      final uri = Uri.parse("$baseUrl/upload").replace(
        queryParameters: skipServerCnn ? {"skip_cnn": "true"} : null,
      );

      // =====================================
      // MULTIPART REQUEST
      // =====================================

      final request = http.MultipartRequest("POST", uri);

      // =====================================
      // HEADER
      // =====================================

      request.headers.addAll({"Accept": "application/json"});

      if (withAuth) {
        final token = (await TokenStorage.getAccessToken())?.trim();
        if (token == null || token.isEmpty) {
          throw const UnauthorizedException(
            "Sesi login tidak ditemukan. Silakan masuk kembali.",
          );
        }
        request.headers["Authorization"] = "Bearer $token";
      }

      final fileBytes = await file.readAsBytes();
      const maxUploadSizeBytes = 10 * 1024 * 1024;
      if (fileBytes.isEmpty) {
        throw const ApiException("File gambar kosong atau tidak dapat dibaca.");
      }
      if (fileBytes.length > maxUploadSizeBytes) {
        throw const ApiException(
          "Ukuran gambar maksimal 10 MB. Kompres gambar atau pilih foto yang lebih kecil.",
        );
      }
      final fileName = file.name.trim().isNotEmpty ? file.name : "upload.jpg";

      final multipartFile = http.MultipartFile.fromBytes(
        "image",
        fileBytes,
        filename: fileName,
      );

      request.files.add(multipartFile);

      debugPrint("========================================");
      debugPrint("UPLOAD IMAGE REQUEST");
      debugPrint("FILE PATH: ${file.path}");
      debugPrint("FILE NAME: $fileName");
      debugPrint("FILE SIZE: ${fileBytes.length} bytes");
      debugPrint("FILES COUNT: ${request.files.length}");
      debugPrint("FIELD NAME: image");

      // =====================================
      // SEND
      // =====================================

      final streamedResponse = await request.send().timeout(uploadTimeout);

      final response = await http.Response.fromStream(streamedResponse);

      debugPrint("UPLOAD IMAGE STATUS: ${response.statusCode}");
      debugPrint("UPLOAD IMAGE RESPONSE");
      debugPrint(response.body);
      debugPrint("========================================");

      return _processResponse(response);
    } on TimeoutException {
      throw ApiException(
        "Upload gambar timeout. Coba kompres gambar atau periksa koneksi ke server.",
      );
    } on http.ClientException {
      throw ApiException("Upload gagal karena server tidak terjangkau.");
    } on FormatException {
      throw const ApiException("Respons upload dari server tidak valid.");
    } catch (e) {
      debugPrint("UPLOAD IMAGE ERROR: $e");
      rethrow;
    }
  }

  // =========================================
  // REFRESH TOKEN
  // =========================================

  Future<bool> refreshToken() async {
    // Backend-v2 saat ini belum menyediakan endpoint /auth/refresh.
    // Method tetap dipertahankan agar kompatibel dengan kode lama, tetapi tidak
    // melakukan request palsu yang bisa membingungkan debugging.
    return false;
  }

  // =========================================
  // RESPONSE HANDLER
  // =========================================

  dynamic _processResponse(http.Response response) {
    final dynamic decodedBody;

    try {
      decodedBody = response.body.trim().isEmpty
          ? <String, dynamic>{}
          : jsonDecode(response.body);
    } on FormatException {
      final fallback = _statusFallback(response.statusCode);
      if (response.statusCode == 401) {
        throw UnauthorizedException(fallback);
      }
      if (response.statusCode >= 400) {
        // Reverse proxy sering mengembalikan HTML/plain text pada 5xx. Tetap
        // tampilkan pesan yang dapat dipahami, bukan error parsing JSON.
        throw ApiException(fallback);
      }
      throw ApiException(
        "Server mengirim respons bukan JSON (HTTP ${response.statusCode}).",
      );
    }

    final body = decodedBody is Map<String, dynamic>
        ? decodedBody
        : <String, dynamic>{"data": decodedBody};

    if (response.statusCode >= 200 && response.statusCode < 300) {
      return body;
    }

    final fallback = _statusFallback(response.statusCode);
    final message = _extractErrorMessage(body, fallback);
    if (response.statusCode == 401) {
      throw UnauthorizedException(message);
    }

    throw ApiException(message);
  }


  String _statusFallback(int statusCode) {
    return switch (statusCode) {
      400 => "Permintaan tidak valid.",
      401 => "Sesi login berakhir. Silakan masuk kembali.",
      403 => "Anda tidak memiliki izin untuk melakukan tindakan ini.",
      404 => "Data atau layanan yang diminta tidak ditemukan.",
      409 => "Data bertentangan dengan kondisi terbaru. Silakan muat ulang.",
      413 => "Ukuran data yang dikirim terlalu besar.",
      422 => "Data yang dikirim belum lengkap atau tidak valid.",
      429 => "Terlalu banyak permintaan. Tunggu sebentar lalu coba lagi.",
      500 => "Terjadi kesalahan pada server.",
      502 => "Layanan sedang tidak tersedia. Tunggu sebentar lalu coba lagi.",
      503 => "Layanan sedang tidak tersedia. Tunggu sebentar lalu coba lagi.",
      504 => "Layanan sedang tidak tersedia. Tunggu sebentar lalu coba lagi.",
      _ => "Permintaan gagal (HTTP $statusCode).",
    };
  }

  String _extractErrorMessage(Map<String, dynamic> body, String fallback) {
    final message = body["message"] ?? body["error"] ?? body["detail"];
    if (message is String && message.trim().isNotEmpty) {
      return message;
    }

    return fallback;
  }
}

class ApiException implements Exception {
  final String message;

  const ApiException(this.message);

  @override
  String toString() => message;
}

class UnauthorizedException implements Exception {
  final String message;

  const UnauthorizedException(this.message);

  @override
  String toString() => message;
}
