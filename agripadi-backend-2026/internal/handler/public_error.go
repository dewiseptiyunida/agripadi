package handler

import "strings"

// publicErrorMessage mengubah kesalahan internal menjadi pesan singkat yang
// aman dan mudah dipahami pengguna. Rincian teknis tetap berada di log server.
func publicErrorMessage(err error, fallback string) string {
	if err == nil {
		return fallback
	}

	message := strings.ToLower(strings.TrimSpace(err.Error()))

	switch {
	case strings.Contains(message, "conversation not found"),
		strings.Contains(message, "riwayat pemeriksaan tidak ditemukan"):
		return "Riwayat pemeriksaan tidak ditemukan."
	case strings.Contains(message, "conversation ids are required"):
		return "Pilih sedikitnya satu riwayat pemeriksaan."
	case strings.Contains(message, "image attachment is required"),
		strings.Contains(message, "attachment url is required"):
		return "Pilih foto terlebih dahulu."
	case strings.Contains(message, "image classification"),
		strings.Contains(message, "detection candidates are required"),
		strings.Contains(message, "detections are required"):
		return "Foto belum dapat diperiksa. Gunakan foto yang lebih jelas lalu coba lagi."
	case strings.Contains(message, "no resolved diagnosis"):
		return "Gejala belum cukup cocok. Periksa kembali kondisi tanaman."
	case strings.Contains(message, "user id is required"),
		strings.Contains(message, "unauthorized"):
		return "Sesi login berakhir. Silakan masuk kembali."
	default:
		return fallback
	}
}
