package firmware

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestStorageChecksumAndLimit(t *testing.T) {
	storage := NewStorage(t.TempDir(), 4)
	file, err := storage.Save("id", strings.NewReader("test"))
	if err != nil {
		t.Fatal(err)
	}
	if file.Size != 4 || file.Checksum != "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08" {
		t.Fatalf("unexpected file: %+v", file)
	}
	if _, err := os.Stat(storage.root + "/id.bin"); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.Save("large", strings.NewReader("12345")); !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("error=%v", err)
	}
}
