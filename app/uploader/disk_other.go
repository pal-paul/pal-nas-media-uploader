//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package uploader

import "fmt"

func availableDiskBytes(string) (int64, error) {
	return 0, fmt.Errorf("disk space checks are unsupported on this platform")
}
