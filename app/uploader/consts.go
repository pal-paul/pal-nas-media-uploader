package uploader

const (
	chunkSize            = int64(8 * 1024 * 1024)
	defaultMaxUploadSize = int64(100 * 1024 * 1024 * 1024)
	maxCreateRequestSize = int64(64 * 1024)
	metadataFilename     = "upload.json"
)
