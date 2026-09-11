package geph

import "testing"

func TestValidateGephControlAddress(t *testing.T) {
	for _, address := range []string{"127.0.0.1:9913", "[::1]:9913"} {
		if err := validateGephControlAddress(address); err != nil {
			t.Fatalf("expected %s to be valid: %v", address, err)
		}
	}
	for _, address := range []string{
		"localhost:9913",
		"0.0.0.0:9913",
		"192.0.2.1:9913",
		"127.0.0.1:0",
		"127.0.0.1:65536",
		"127.0.0.1:-1",
		"127.0.0.1",
	} {
		if err := validateGephControlAddress(address); err == nil {
			t.Fatalf("expected %s to be invalid", address)
		}
	}
}
