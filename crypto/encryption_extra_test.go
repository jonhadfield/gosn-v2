package crypto

import (
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
)

func TestHexDecodeStringsValid(t *testing.T) {
	in := "00112233aabbccddeeff001122334455"
	out, err := hexDecodeStrings(in, len(in)/2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hex.EncodeToString(out) != strings.ToLower(in) {
		t.Fatalf("expected %s got %s", in, hex.EncodeToString(out))
	}
}

func TestHexDecodeStringsInvalid(t *testing.T) {
	if _, err := hexDecodeStrings("zz", 1); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGenerateItemKeyLength(t *testing.T) {
	key := GenerateItemKey(32)
	if len(key) != 32 {
		t.Fatalf("expected length 32 got %d", len(key))
	}
	key2 := GenerateItemKey(32)
	if key == key2 {
		t.Fatalf("expected different keys")
	}
}

func TestGenerateNonceLength(t *testing.T) {
	n := GenerateNonce()
	if len(n) != NonceSizeX {
		t.Fatalf("expected nonce length %d got %d", NonceSizeX, len(n))
	}
}

func TestPadToAESBlockSize(t *testing.T) {
	b := []byte("1234567")
	pb := padToAESBlockSize(b)
	if len(pb)%16 != 0 {
		t.Fatalf("expected padded length multiple of 16 got %d", len(pb))
	}
	padding := pb[len(pb)-1]
	if int(padding) != len(pb)-len(b) {
		t.Fatalf("unexpected padding byte")
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := []byte("0123456789abcdef")
	msg := "hello world"
	ct := Encrypt(key, msg)
	pt, err := Decrypt(key, ct)
	if err != nil {
		t.Fatalf("decrypt error: %v", err)
	}
	if pt != msg {
		t.Fatalf("expected %s got %s", msg, pt)
	}
}

func TestDecryptErrors(t *testing.T) {
	key := []byte("0123456789abcdef")
	// invalid base64
	if _, err := Decrypt(key, "!"); err == nil {
		t.Fatalf("expected error for invalid base64")
	}
	// too short ciphertext
	if _, err := Decrypt(key, base64.URLEncoding.EncodeToString([]byte("short"))); err == nil {
		t.Fatalf("expected error for short ciphertext")
	}
}

func TestEncryptPlaintextTooLong(t *testing.T) {
	defer func() { _ = recover() }()
	key := []byte("0123456789abcdef0123456789abcdef")
	long := make([]byte, MaxPlaintextSize+1)
	Encrypt(key, string(long))
	t.Fatalf("expected panic for long plaintext")
}
