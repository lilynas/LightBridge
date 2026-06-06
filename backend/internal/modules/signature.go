package modules

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
)

const SignatureFilename = "signature.sig"

type SignatureVerifier interface {
	Verify(message []byte, signature []byte) error
}

type SignatureVerifierFunc func(message []byte, signature []byte) error

func (fn SignatureVerifierFunc) Verify(message []byte, signature []byte) error {
	return fn(message, signature)
}

type Ed25519SignatureVerifier struct {
	publicKey ed25519.PublicKey
}

func NewEd25519SignatureVerifier(publicKey ed25519.PublicKey) (*Ed25519SignatureVerifier, error) {
	if len(publicKey) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("ed25519 public key must be %d bytes", ed25519.PublicKeySize)
	}
	return &Ed25519SignatureVerifier{publicKey: append(ed25519.PublicKey(nil), publicKey...)}, nil
}

func NewEd25519SignatureVerifierFromFile(path string) (*Ed25519SignatureVerifier, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read module signature public key: %w", err)
	}
	publicKey, err := parseEd25519PublicKey(content)
	if err != nil {
		return nil, err
	}
	return NewEd25519SignatureVerifier(publicKey)
}

func (v *Ed25519SignatureVerifier) Verify(message []byte, signature []byte) error {
	if v == nil || len(v.publicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("module signature verifier is not configured")
	}
	parsedSignature, err := parseEd25519Signature(signature)
	if err != nil {
		return err
	}
	if !ed25519.Verify(v.publicKey, message, parsedSignature) {
		return fmt.Errorf("module signature verification failed")
	}
	return nil
}

func parseEd25519PublicKey(content []byte) (ed25519.PublicKey, error) {
	if len(content) == ed25519.PublicKeySize {
		return append(ed25519.PublicKey(nil), content...), nil
	}
	text := strings.TrimSpace(string(content))
	if text == "" {
		return nil, fmt.Errorf("module signature public key is empty")
	}
	if block, _ := pem.Decode([]byte(text)); block != nil {
		if len(block.Bytes) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("ed25519 PEM public key must contain %d raw bytes", ed25519.PublicKeySize)
		}
		return append(ed25519.PublicKey(nil), block.Bytes...), nil
	}
	text = strings.TrimPrefix(text, "ed25519:")
	if decoded, ok := decodeFixedBytes(text, ed25519.PublicKeySize); ok {
		return append(ed25519.PublicKey(nil), decoded...), nil
	}
	return nil, fmt.Errorf("ed25519 public key must be raw, hex, base64, or raw PEM bytes")
}

func parseEd25519Signature(content []byte) ([]byte, error) {
	if len(content) == ed25519.SignatureSize {
		return append([]byte(nil), content...), nil
	}
	text := strings.TrimSpace(string(content))
	if text == "" {
		return nil, fmt.Errorf("module signature is empty")
	}
	if decoded, ok := decodeFixedBytes(text, ed25519.SignatureSize); ok {
		return decoded, nil
	}
	return nil, fmt.Errorf("ed25519 signature must be %d raw bytes, hex, or base64", ed25519.SignatureSize)
}

func decodeFixedBytes(text string, want int) ([]byte, bool) {
	if decoded, err := hex.DecodeString(text); err == nil && len(decoded) == want {
		return decoded, true
	}
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	for _, encoding := range encodings {
		if decoded, err := encoding.DecodeString(text); err == nil && len(decoded) == want {
			return decoded, true
		}
	}
	return nil, false
}
