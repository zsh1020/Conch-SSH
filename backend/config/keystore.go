package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
)

// machineKey derives a 32-byte AES key from machine-specific info.
// Uses a combination of hostname and user home directory path.
func machineKey() ([]byte, error) {
	hostname, _ := os.Hostname()
	home, _ := os.UserHomeDir()
	// ⚠ 种子前缀 "gssh" 是历史常量，务必保持不变：改了会改变派生密钥，
	// 导致所有已保存的密码无法解密。与对外品牌名 Conch 无关。
	seed := fmt.Sprintf("gssh:%s:%s", hostname, home)
	h := sha256.Sum256([]byte(seed))
	return h[:], nil
}

// Encrypt encrypts plaintext with AES-256-GCM and returns a base64 string.
func Encrypt(plaintext string) (string, error) {
	key, err := machineKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt decrypts a base64-encoded AES-256-GCM ciphertext.
func Decrypt(ciphertext string) (string, error) {
	key, err := machineKey()
	if err != nil {
		return "", err
	}
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	plaintext, err := gcm.Open(nil, data[:nonceSize], data[nonceSize:], nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
