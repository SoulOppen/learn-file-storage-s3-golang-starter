package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"

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
	mediaType := headerFile.Header.Get("Content-Type")
	image, err := io.ReadAll(file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Can't read file", err)
		return
	}
	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Not found video", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Not found video", err)
		return
	}
	sImage := base64.StdEncoding.EncodeToString(image)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mediaType, sImage)
	video.ThumbnailURL = &dataURL
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Not Updated video", err)
		return
	}
	respondWithJSON(w, http.StatusOK, video)
}
