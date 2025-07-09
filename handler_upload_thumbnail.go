package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	const max = 10 << 20
	err = r.ParseMultipartForm(max)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid transform", err)
		return
	}
	file, headerFile, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Not file", err)
		return
	}
	defer file.Close()
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Can't read file", err)
		return
	}
	mediaType := headerFile.Header.Get("Content-Type")
	exts, err := mime.ExtensionsByType(mediaType)
	if err != nil || len(exts) == 0 {
		respondWithError(w, http.StatusBadRequest, "Invalid media type", err)
		return
	}
	ext := exts[0]
	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Not found video", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Not found video", err)
		return
	}
	path := filepath.Join(cfg.assetsRoot, fmt.Sprintf("%s%s", videoIDString, ext))
	newFile, err := os.Create(path)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Not found video", err)
		return
	}
	defer newFile.Close()
	_, err = io.Copy(newFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to save file", err)
		return
	}
	url := fmt.Sprintf("http://localhost:%s/%s", cfg.port, path)
	video.ThumbnailURL = &url
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Not Updated video", err)
		return
	}
	fmt.Println(*video.ThumbnailURL)
	respondWithJSON(w, http.StatusOK, video)
}
