package uploader

const (
	chunkSize             = int64(8 * 1024 * 1024)
	defaultMaxUploadSize  = int64(100 * 1024 * 1024 * 1024)
	defaultMaxPendingSize = int64(100 * 1024 * 1024 * 1024)
	defaultMaxStorageSize = int64(1024 * 1024 * 1024 * 1024)
	defaultMinDiskFree    = int64(2 * 1024 * 1024 * 1024)
	defaultMaxActive      = 10
	maxCreateRequestSize  = int64(64 * 1024)
	metadataFilename      = "upload.json"
)
