//go:build unix

package operations

import "golang.org/x/sys/unix"

func availableDiskBytes(path string) (int64, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0, err
	}
	available := uint64(stat.Bavail) * uint64(stat.Bsize)
	const maxInt64 = int64(^uint64(0) >> 1)
	if available > uint64(maxInt64) {
		return maxInt64, nil
	}
	return int64(available), nil
}
