package host

import (
	"context"
	"errors"
	"io/fs"
	"strings"
)

type Device struct {
	Path      string
	Block     bool
	Signature string
}

func ProbeDevice(ctx context.Context, e Exec, statFn func(string) (fs.FileInfo, error), path string) (Device, error) {
	info, err := statFn(path)
	if err != nil {
		return Device{}, err
	}
	mode := info.Mode()
	block := mode&fs.ModeDevice != 0 && mode&fs.ModeCharDevice == 0
	if !block {
		return Device{Path: path}, nil
	}
	out, err := e.Run(ctx, "blkid", "-p", "-o", "value", "-s", "TYPE", path)
	signature := strings.TrimSpace(out)
	var exit *ExitError
	if err != nil && (!errors.As(err, &exit) || signature != "") {
		return Device{}, err
	}
	return Device{Path: path, Block: true, Signature: signature}, nil
}
