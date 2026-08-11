package security

import "testing"

func TestCipherRoundTrip(t *testing.T) {
	cipher, err := NewCipher("MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	if err != nil {
		t.Fatalf("NewCipher() error = %v", err)
	}
	encrypted, err := cipher.Encrypt("secret-password")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if string(encrypted) == "secret-password" {
		t.Fatal("plaintext was not encrypted")
	}
	decrypted, err := cipher.Decrypt(encrypted)
	if err != nil || decrypted != "secret-password" {
		t.Fatalf("Decrypt() = %q, %v", decrypted, err)
	}
}
