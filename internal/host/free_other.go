//go:build !linux

package host

import "errors"

func FreeBytes(string) (uint64, error) {
	return 0, errors.New("free space check needs linux")
}
