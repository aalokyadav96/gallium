package feed

func createDefaultPoster(originalFilePath, uploadDir, uniqueID string) error {
	defaultPosterPath := generateFilePath(uploadDir, uniqueID, "jpg")
	return CreatePoster(originalFilePath, defaultPosterPath, "00:00:01")
}
