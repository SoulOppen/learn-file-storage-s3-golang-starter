package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error to get token", err)
		return
	}
	const max = 1 << 30
	r.Body = http.MaxBytesReader(w, r.Body, max)
	videoID := r.PathValue("videoID")
	uVideoId, err := uuid.Parse(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid transform", err)
		return
	}
	video, err := cfg.db.GetVideo(uVideoId)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Video not found", err)
		return
	}
	userId, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Can't get id", err)
		return
	}
	if userId != video.UserID {
		respondWithError(w, http.StatusUnauthorized, "Not authorize", err)
		return
	}
	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Can't get file", err)
		return
	}
	defer file.Close()
	mediaType := header.Header.Get("Content-Type")
	exts, _, err := mime.ParseMediaType(mediaType)
	if err != nil || len(exts) == 0 {
		respondWithError(w, http.StatusBadRequest, "Invalid media type", err)
		return
	}

	if exts != "video/mp4" {
		respondWithError(w, http.StatusUnsupportedMediaType, "Only MP4 videos are allowed", err)
		return
	}
	f, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Can't create file", err)
		return
	}

	if _, err := io.Copy(f, file); err != nil {
		respondWithError(w, http.StatusBadRequest, "Can't copy data to file", err)
		return
	}

	if _, err := f.Seek(0, 0); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Can't rewind file", err)
		return
	}
	mask := make([]byte, 32)
	_, err = rand.Read(mask)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Can't generate random filename", err)
		return
	}
	maskEncode := base64.RawURLEncoding.EncodeToString(mask)
	processedPath, err := processVideoForFastStart(f.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Can't process video", err)
		return
	}
	defer os.Remove(processedPath)

	str, err := getVideoAspectRatio(processedPath)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Can't get file info", err)
		return
	}
	var strKey string
	switch str {
	case "16:9":
		strKey = "landscape"
	case "9:16":
		strKey = "portrait"
	default:
		strKey = "other"
	}
	key := fmt.Sprintf("video/%s/%s.mp4", strKey, maskEncode)
	processedFile, err := os.Open(processedPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Can't open processed file", err)
		return
	}
	defer processedFile.Close()
	_, err = cfg.s3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(cfg.s3Bucket),
		Key:         aws.String(key),
		Body:        processedFile,
		ContentType: aws.String(mediaType),
	})
	if err != nil {
		http.Error(w, "Failed to upload to S3", http.StatusInternalServerError)
		return
	}
	defer os.Remove(f.Name())
	defer f.Close()
	path := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, key)
	video.VideoURL = &path
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Not Updated video", err)
		return
	}
	respondWithJSON(w, http.StatusOK, video)
}
