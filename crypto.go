package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
)

const encryptedHeader = "MOLOTOV-ENCRYPTED-V1\n"
const maxImport = 46 * 1024 * 1024

func newKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}
func keyCipher(key string) (cipher.AEAD, error) {
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(key))
	if err != nil || len(b) != 32 {
		return nil, fmt.Errorf("Provide a base64-encoded 32-byte key, or generate one")
	}
	block, err := aes.NewCipher(b)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
func encodeOutput(text, key string) (string, error) {
	if len(text) > maxReport {
		return "", fmt.Errorf("Report exceeds 32 MiB")
	}
	if strings.TrimSpace(key) == "" {
		return text, nil
	}
	g, err := keyCipher(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := g.Seal(nonce, nonce, []byte(text), []byte(encryptedHeader))
	return encryptedHeader + base64.StdEncoding.EncodeToString(sealed), nil
}
func decodeInput(text, key string) (string, error) {
	if len(text) > maxImport {
		return "", fmt.Errorf("Input exceeds 46 MiB")
	}
	text = strings.TrimPrefix(text, "\ufeff")
	if !strings.HasPrefix(text, encryptedHeader) {
		if len(text) > maxReport {
			return "", fmt.Errorf("Plain report exceeds 32 MiB")
		}
		return text, nil
	}
	g, err := keyCipher(key)
	if err != nil {
		return "", err
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(strings.TrimPrefix(text, encryptedHeader)))
	if err != nil || len(data) < g.NonceSize()+g.Overhead() {
		return "", fmt.Errorf("Invalid encrypted report")
	}
	plain, err := g.Open(nil, data[:g.NonceSize()], data[g.NonceSize():], []byte(encryptedHeader))
	if err != nil {
		return "", fmt.Errorf("Cannot decrypt: wrong key or modified report")
	}
	if len(plain) > maxReport {
		return "", fmt.Errorf("Report exceeds 32 MiB")
	}
	return string(plain), nil
}
