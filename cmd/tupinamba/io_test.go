package main

import (
	"errors"
	"testing"
)

type testReadRoot struct {
	data     []byte
	readErr  error
	closeErr error
}

func (r testReadRoot) ReadFile(string) ([]byte, error) { return r.data, r.readErr }
func (r testReadRoot) Close() error                    { return r.closeErr }

func TestReadPathFileReturnsRootAndReadErrors(t *testing.T) {
	previous := openReadRoot
	t.Cleanup(func() { openReadRoot = previous })
	openErr := errors.New("open root failed")
	openReadRoot = func(string) (readRoot, error) { return nil, openErr }
	if _, err := readPathFile("file"); !errors.Is(err, openErr) {
		t.Fatalf("open error = %v", err)
	}

	readErr := errors.New("read failed")
	openReadRoot = func(string) (readRoot, error) { return testReadRoot{readErr: readErr}, nil }
	if _, err := readPathFile("file"); !errors.Is(err, readErr) {
		t.Fatalf("read error = %v", err)
	}
}

func TestReadPathFileReturnsCloseError(t *testing.T) {
	previous := openReadRoot
	t.Cleanup(func() { openReadRoot = previous })
	closeErr := errors.New("close root failed")
	openReadRoot = func(string) (readRoot, error) {
		return testReadRoot{data: []byte("input"), closeErr: closeErr}, nil
	}
	if data, err := readPathFile("file"); !errors.Is(err, closeErr) || string(data) != "input" {
		t.Fatalf("readPathFile() = %q, %v", data, err)
	}
}
