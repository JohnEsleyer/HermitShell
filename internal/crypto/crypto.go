package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

const BlockSize = 16

func DeriveKey(password string) []byte {
	hash := sha256.Sum256([]byte(password))
	return hash[:]
}

func pkcs7Pad(data []byte) []byte {
	padLen := BlockSize - (len(data) % BlockSize)
	pad := make([]byte, padLen)
	for i := range pad {
		pad[i] = byte(padLen)
	}
	return append(data, pad...)
}

func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}
	padLen := int(data[len(data)-1])
	if padLen > BlockSize || padLen == 0 {
		return nil, fmt.Errorf("invalid padding")
	}
	if len(data) < padLen {
		return nil, fmt.Errorf("invalid padding length")
	}
	for i := 0; i < padLen; i++ {
		if data[len(data)-1-i] != byte(padLen) {
			return nil, fmt.Errorf("invalid padding")
		}
	}
	return data[:len(data)-padLen], nil
}

// Encrypt encrypts using AES-CBC with PKCS7 padding.
// Output format: base64(IV + ciphertext)
func Encrypt(plaintext string, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	iv := make([]byte, BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}

	padded := pkcs7Pad([]byte(plaintext))
	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, padded)

	return base64.StdEncoding.EncodeToString(append(iv, ciphertext...)), nil
}

// Decrypt handles both old (GCM) and new (CBC) encryption formats.
// Old format: "enc:" prefix (GCM with 12-byte nonce)
// New format: "cbc:" prefix (CBC with PKCS7 padding)
func Decrypt(cryptoText string, key []byte) (string, error) {
	data := cryptoText

	// Extract prefix
	if strings.HasPrefix(cryptoText, "cbc:") {
		data = cryptoText[4:]
	} else if strings.HasPrefix(cryptoText, "enc:") {
		// Check if it's old GCM format or new format without prefix
		decoded, err := base64.StdEncoding.DecodeString(data[4:])
		if err != nil {
			// Try treating as raw base64 (no prefix)
			decoded, err = base64.StdEncoding.DecodeString(cryptoText)
			if err != nil {
				return "", fmt.Errorf("invalid base64")
			}
			return decryptCBC(decoded, key)
		}
		// If decoded data is less than 28 bytes, it's likely old GCM (12 IV + small ciphertext)
		if len(decoded) < 28 {
			return "", fmt.Errorf("legacy GCM format no longer supported, please clear chat history")
		}
		// Otherwise it's new CBC format
		return decryptCBC(decoded, key)
	} else {
		// No prefix, try as raw base64
		decoded, err := base64.StdEncoding.DecodeString(cryptoText)
		if err != nil {
			return "", fmt.Errorf("invalid base64")
		}
		return decryptCBC(decoded, key)
	}

	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", fmt.Errorf("invalid base64: %w", err)
	}

	return decryptCBC(decoded, key)
}

func decryptCBC(data []byte, key []byte) (string, error) {
	if len(data) < BlockSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	iv := data[:BlockSize]
	ciphertext := data[BlockSize:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	plaintext := make([]byte, len(ciphertext))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(plaintext, ciphertext)

	unpadded, err := pkcs7Unpad(plaintext)
	if err != nil {
		return "", fmt.Errorf("CBC decryption failed: %w", err)
	}

	return string(unpadded), nil
}
