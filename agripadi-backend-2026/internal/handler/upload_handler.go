package handler

import (
	"fmt"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gustian305/backend/internal/dto"
	"github.com/gustian305/backend/internal/service/cnn"
	"github.com/gustian305/backend/logger"
)

type UploadHandler struct {
	uploadDir string
	cnn       *cnn.Service
	cnnMode   string
}

func NewUploadHandler(uploadDir string, cnnService *cnn.Service, cnnMode string) *UploadHandler {
	cnnMode = strings.ToLower(strings.TrimSpace(cnnMode))
	if cnnMode == "" {
		cnnMode = "server_api"
	}

	return &UploadHandler{
		uploadDir: uploadDir,
		cnn:       cnnService,
		cnnMode:   cnnMode,
	}
}

// UploadImage godoc
// @Summary Upload gambar hama padi
// @Description Mengunggah gambar hama padi. Jika mode CNN server aktif, backend dapat mengembalikan kandidat deteksi.
// @Tags Upload
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param image formData file true "File gambar jpg, jpeg, png, atau webp maksimal 10MB"
// @Param skip_cnn query bool false "Lewati klasifikasi CNN server"
// @Success 200 {object} dto.UploadImageResponse
// @Failure 400 {object} dto.APIErrorResponse
// @Failure 401 {object} dto.APIErrorResponse
// @Failure 500 {object} dto.APIErrorResponse
// @Router /api/upload [post]
func (h *UploadHandler) UploadImage(c *gin.Context) {
	startedAt := time.Now()
	operation := "UploadImage"

	// =====================================
	// GET FILE
	// =====================================

	file, err := c.FormFile("image")

	if err != nil {
		logger.Failure(
			"upload",
			operation,
			startedAt,
			err,
		)

		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Pilih foto terlebih dahulu.",
		})

		return
	}

	logger.Request(
		"upload",
		operation,
		slog.String("file_extension", strings.ToLower(filepath.Ext(file.Filename))),
		slog.Int64("file_size", file.Size),
	)

	// =====================================
	// VALIDATE EXTENSION
	// =====================================

	ext := strings.ToLower(
		filepath.Ext(file.Filename),
	)

	allowed := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".webp": true,
	}

	if !allowed[ext] {
		logger.Failure(
			"upload",
			operation,
			startedAt,
			fmt.Errorf("invalid image extension: %s", ext),
		)

		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Format foto harus JPG, JPEG, PNG, atau WEBP.",
		})

		return
	}

	// =====================================
	// VALIDATE SIZE
	// =====================================

	const MaxFileSize = 10 << 20 // 10MB

	if file.Size > MaxFileSize {
		logger.Failure(
			"upload",
			operation,
			startedAt,
			fmt.Errorf("image too large: %d", file.Size),
		)

		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Ukuran foto terlalu besar. Maksimal 10 MB.",
		})

		return
	}

	if err := validateImageContent(file); err != nil {
		logger.Failure(
			"upload",
			operation,
			startedAt,
			err,
		)

		c.JSON(http.StatusBadRequest, gin.H{
			"error": "File yang dipilih bukan foto JPG, PNG, atau WEBP yang dapat dibaca.",
		})

		return
	}

	// =====================================
	// CREATE DIRECTORY
	// =====================================

	if err := os.MkdirAll(
		h.uploadDir,
		0750,
	); err != nil {
		logger.Failure(
			"upload",
			operation,
			startedAt,
			err,
		)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Foto belum berhasil disimpan. Coba lagi beberapa saat.",
		})

		return
	}

	// =====================================
	// GENERATE FILE NAME
	// =====================================

	filename := fmt.Sprintf(
		"%d_%s%s",
		time.Now().Unix(),
		uuid.New().String(),
		ext,
	)

	dst := filepath.Join(
		h.uploadDir,
		filename,
	)

	// =====================================
	// SAVE FILE
	// =====================================

	if err := c.SaveUploadedFile(
		file,
		dst,
	); err != nil {
		logger.Failure(
			"upload",
			operation,
			startedAt,
			err,
		)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Foto belum berhasil dikirim. Coba lagi.",
		})

		return
	}

	imageURL :=
		"/uploads/" + filename

	detections := []dto.DetectionCandidateRequest{}
	querySkipCNN := strings.EqualFold(c.Query("skip_cnn"), "true") ||
		strings.EqualFold(c.Query("cnn"), "false")
	skipCNN := querySkipCNN || h.cnnMode == "on_device_flutter"
	cnnSource := h.cnnMode
	if cnnSource == "" {
		cnnSource = "server_api"
	}

	if skipCNN {
		cnnSource = "on_device_flutter"
	} else if h.cnn != nil {
		result, err := h.cnn.ClassifyFile(
			c.Request.Context(),
			dst,
		)

		if err != nil {
			logger.Failure(
				"upload",
				"ClassifyUploadedImage",
				startedAt,
				err,
				slog.String("image_url", imageURL),
			)
		} else {
			detections = result
		}
	} else {
		cnnSource = "server_api_unavailable"
	}

	logger.Response(
		"upload",
		operation,
		startedAt,
		slog.String("image_url", imageURL),
		slog.Bool("skip_cnn", skipCNN),
		slog.String("cnn_source", cnnSource),
		slog.Int("detection_count", len(detections)),
	)

	logUploadCNNDetections(
		detections,
		slog.Bool("skip_cnn", skipCNN),
		slog.String("cnn_source", cnnSource),
	)

	c.JSON(http.StatusOK, gin.H{
		"message":    "Foto berhasil dikirim.",
		"url":        imageURL,
		"cnn_source": cnnSource,
		"detections": detections,
	})
}

func logUploadCNNDetections(detections []dto.DetectionCandidateRequest, attrs ...slog.Attr) {
	// Hasil rinci tidak ditulis ke log untuk menjaga privasi data pemeriksaan.
	if len(detections) == 0 {
		return
	}

	baseAttrs := []slog.Attr{
		slog.Int("detection_count", len(detections)),
		slog.String("top_label", detections[0].Label),
		slog.Float64("top_confidence", detections[0].Confidence),
	}

	logger.InfoPayload(
		"cnn",
		"UploadImage",
		"ringkasan hasil pemeriksaan foto",
		append(baseAttrs, attrs...)...,
	)
}

func validateImageContent(fileHeader *multipart.FileHeader) error {
	file, err := fileHeader.Open()
	if err != nil {
		return err
	}
	defer file.Close()

	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && n == 0 {
		return err
	}

	contentType := http.DetectContentType(buffer[:n])
	switch contentType {
	case "image/jpeg", "image/png", "image/webp":
		return nil
	default:
		return fmt.Errorf("unsupported image content type: %s", contentType)
	}
}
