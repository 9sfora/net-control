package firmware

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var ErrFileTooLarge = errors.New("firmware file is too large")

type StoredFile struct {
	Path     string
	Size     int64
	Checksum string
}
type Storage struct {
	root    string
	maxSize int64
}

func NewStorage(root string, maxSize int64) *Storage { return &Storage{root: root, maxSize: maxSize} }
func (s *Storage) Save(id string, reader io.Reader) (StoredFile, error) {
	if err := os.MkdirAll(s.root, 0o750); err != nil {
		return StoredFile{}, fmt.Errorf("create firmware directory: %w", err)
	}
	temporary, err := os.CreateTemp(s.root, "upload-*")
	if err != nil {
		return StoredFile{}, fmt.Errorf("create firmware temporary file: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	hash := sha256.New()
	size, copyErr := io.Copy(io.MultiWriter(temporary, hash), io.LimitReader(reader, s.maxSize+1))
	closeErr := temporary.Close()
	if copyErr != nil {
		return StoredFile{}, fmt.Errorf("write firmware: %w", copyErr)
	}
	if closeErr != nil {
		return StoredFile{}, fmt.Errorf("close firmware: %w", closeErr)
	}
	if size > s.maxSize {
		return StoredFile{}, ErrFileTooLarge
	}
	if err := os.Chmod(temporaryName, 0o640); err != nil {
		return StoredFile{}, err
	}
	relative := id + ".bin"
	if err := os.Rename(temporaryName, filepath.Join(s.root, relative)); err != nil {
		return StoredFile{}, fmt.Errorf("store firmware: %w", err)
	}
	return StoredFile{Path: relative, Size: size, Checksum: hex.EncodeToString(hash.Sum(nil))}, nil
}
func (s *Storage) Remove(path string) error {
	if path == "" {
		return nil
	}
	return os.Remove(filepath.Join(s.root, filepath.Base(path)))
}
