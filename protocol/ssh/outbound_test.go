package ssh

import (
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestValidateAlgorithms(t *testing.T) {
	supported := ssh.SupportedAlgorithms()
	insecure := ssh.InsecureAlgorithms()

	t.Run("empty list returns nil", func(t *testing.T) {
		result, err := validateAlgorithms(nil, supported.Ciphers, insecure.Ciphers, "cipher")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != nil {
			t.Fatalf("expected nil, got %v", result)
		}
	})

	t.Run("valid supported ciphers", func(t *testing.T) {
		input := []string{"aes128-gcm@openssh.com", "chacha20-poly1305@openssh.com"}
		result, err := validateAlgorithms(input, supported.Ciphers, insecure.Ciphers, "cipher")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 2 {
			t.Fatalf("expected 2 results, got %d", len(result))
		}
		if result[0] != "aes128-gcm@openssh.com" || result[1] != "chacha20-poly1305@openssh.com" {
			t.Fatalf("unexpected result: %v", result)
		}
	})

	t.Run("valid insecure ciphers", func(t *testing.T) {
		input := []string{"aes128-cbc", "3des-cbc"}
		result, err := validateAlgorithms(input, supported.Ciphers, insecure.Ciphers, "cipher")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 2 {
			t.Fatalf("expected 2 results, got %d", len(result))
		}
	})

	t.Run("invalid cipher returns error", func(t *testing.T) {
		input := []string{"aes128-gcm@openssh.com", "invalid-cipher-123"}
		_, err := validateAlgorithms(input, supported.Ciphers, insecure.Ciphers, "cipher")
		if err == nil {
			t.Fatal("expected error for invalid cipher")
		}
		expected := "unknown cipher: invalid-cipher-123"
		if err.Error() != expected {
			t.Fatalf("expected error %q, got %q", expected, err.Error())
		}
	})

	t.Run("case normalization", func(t *testing.T) {
		input := []string{"AES128-GCM@OPENSSH.COM"}
		result, err := validateAlgorithms(input, supported.Ciphers, insecure.Ciphers, "cipher")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result[0] != "aes128-gcm@openssh.com" {
			t.Fatalf("expected lowercase, got %q", result[0])
		}
	})

	t.Run("order preserved", func(t *testing.T) {
		input := []string{"aes256-ctr", "aes128-gcm@openssh.com", "chacha20-poly1305@openssh.com"}
		result, err := validateAlgorithms(input, supported.Ciphers, insecure.Ciphers, "cipher")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result[0] != "aes256-ctr" || result[1] != "aes128-gcm@openssh.com" || result[2] != "chacha20-poly1305@openssh.com" {
			t.Fatalf("order not preserved: %v", result)
		}
	})

	t.Run("valid MACs", func(t *testing.T) {
		input := []string{"hmac-sha2-256-etm@openssh.com"}
		result, err := validateAlgorithms(input, supported.MACs, insecure.MACs, "mac")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 1 || result[0] != "hmac-sha2-256-etm@openssh.com" {
			t.Fatalf("unexpected result: %v", result)
		}
	})

	t.Run("invalid MAC returns error", func(t *testing.T) {
		input := []string{"invalid-mac"}
		_, err := validateAlgorithms(input, supported.MACs, insecure.MACs, "mac")
		if err == nil {
			t.Fatal("expected error for invalid mac")
		}
		expected := "unknown mac: invalid-mac"
		if err.Error() != expected {
			t.Fatalf("expected error %q, got %q", expected, err.Error())
		}
	})

	t.Run("valid key exchanges", func(t *testing.T) {
		input := []string{"curve25519-sha256"}
		result, err := validateAlgorithms(input, supported.KeyExchanges, insecure.KeyExchanges, "key exchange")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 1 || result[0] != "curve25519-sha256" {
			t.Fatalf("unexpected result: %v", result)
		}
	})

	t.Run("invalid key exchange returns error", func(t *testing.T) {
		input := []string{"invalid-kex"}
		_, err := validateAlgorithms(input, supported.KeyExchanges, insecure.KeyExchanges, "key exchange")
		if err == nil {
			t.Fatal("expected error for invalid key exchange")
		}
		expected := "unknown key exchange: invalid-kex"
		if err.Error() != expected {
			t.Fatalf("expected error %q, got %q", expected, err.Error())
		}
	})

	t.Run("duplicates passed through", func(t *testing.T) {
		input := []string{"aes128-gcm@openssh.com", "aes128-gcm@openssh.com"}
		result, err := validateAlgorithms(input, supported.Ciphers, insecure.Ciphers, "cipher")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 2 {
			t.Fatalf("expected 2 results (duplicates preserved), got %d", len(result))
		}
	})
}
