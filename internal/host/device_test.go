package host

import (
	"context"
	"errors"
	"io/fs"
	"testing"
	"time"
)

type fakeInfo struct{ mode fs.FileMode }

func (f fakeInfo) Name() string       { return "dev" }
func (f fakeInfo) Size() int64        { return 0 }
func (f fakeInfo) Mode() fs.FileMode  { return f.mode }
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool        { return false }
func (f fakeInfo) Sys() any           { return nil }

func TestProbeDeviceBlockWithoutSignature(t *testing.T) {
	e := &FakeExec{Errors: map[string]error{"blkid -p -o value -s TYPE /dev/sdb": &ExitError{Code: 2}}}
	dev, err := ProbeDevice(context.Background(), e, func(string) (fs.FileInfo, error) { return fakeInfo{fs.ModeDevice}, nil }, "/dev/sdb")
	if err != nil {
		t.Fatal(err)
	}
	if !dev.Block || dev.Signature != "" {
		t.Fatalf("device %+v", dev)
	}
}

func TestProbeDeviceMissingBlkidFails(t *testing.T) {
	e := &FakeExec{Errors: map[string]error{"blkid -p -o value -s TYPE /dev/sdb": errors.New("executable file not found in $PATH")}}
	_, err := ProbeDevice(context.Background(), e, func(string) (fs.FileInfo, error) { return fakeInfo{fs.ModeDevice}, nil }, "/dev/sdb")
	if err == nil {
		t.Fatal("missing blkid binary must be an error")
	}
}

func TestProbeDeviceWithFilesystem(t *testing.T) {
	e := &FakeExec{Responses: map[string]string{"blkid -p -o value -s TYPE /dev/sdb": "ext4\n"}}
	dev, err := ProbeDevice(context.Background(), e, func(string) (fs.FileInfo, error) { return fakeInfo{fs.ModeDevice}, nil }, "/dev/sdb")
	if err != nil {
		t.Fatal(err)
	}
	if dev.Signature != "ext4" {
		t.Fatalf("device %+v", dev)
	}
}

func TestProbeDeviceNotBlock(t *testing.T) {
	e := &FakeExec{}
	dev, err := ProbeDevice(context.Background(), e, func(string) (fs.FileInfo, error) { return fakeInfo{fs.ModeDevice | fs.ModeCharDevice}, nil }, "/dev/null")
	if err != nil {
		t.Fatal(err)
	}
	if dev.Block {
		t.Fatal("char device must not be block")
	}
}
