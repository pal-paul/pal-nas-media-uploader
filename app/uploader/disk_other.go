//go:build !unix

package uploader

import "fmt"

func availableDiskBytes(string) (int64, error) {
	return 0, fmt.Errorf("disk space checks are unsupported on this platform")
}
