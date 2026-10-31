package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	gossh "golang.org/x/crypto/ssh"
)

// GenerateKeyResult holds the paths of the generated key pair.
type GenerateKeyResult struct {
	PrivateKeyPath string `json:"private_key_path"`
	PublicKey      string `json:"public_key"`
}

// GenerateKey generates an SSH key pair and writes it to ~/.ssh/.
// keyType must be "ed25519" or "rsa4096".
func (a *App) GenerateKey(keyType, comment string) (*GenerateKeyResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return nil, err
	}

	var privPEM []byte
	var pubKeyStr string

	switch keyType {
	case "ed25519":
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, err
		}
		privPEM, err = marshalED25519Private(priv)
		if err != nil {
			return nil, err
		}
		sshPub, err := gossh.NewPublicKey(pub)
		if err != nil {
			return nil, err
		}
		pubKeyStr = authorizedKeyLine(sshPub, comment)

	case "rsa4096":
		priv, err := rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			return nil, err
		}
		privPEM = pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(priv),
		})
		sshPub, err := gossh.NewPublicKey(&priv.PublicKey)
		if err != nil {
			return nil, err
		}
		pubKeyStr = authorizedKeyLine(sshPub, comment)

	default:
		return nil, fmt.Errorf("unsupported key type: %s", keyType)
	}

	suffix := map[string]string{"ed25519": "id_ed25519", "rsa4096": "id_rsa4096"}[keyType]
	privPath := uniquePath(filepath.Join(sshDir, suffix))
	pubPath := privPath + ".pub"

	if err := os.WriteFile(privPath, privPEM, 0o600); err != nil {
		return nil, err
	}
	if err := os.WriteFile(pubPath, []byte(pubKeyStr), 0o644); err != nil {
		return nil, err
	}

	return &GenerateKeyResult{PrivateKeyPath: privPath, PublicKey: pubKeyStr}, nil
}

// authorizedKeyLine builds an authorized_keys line, appending an optional comment.
func authorizedKeyLine(pub gossh.PublicKey, comment string) string {
	line := string(gossh.MarshalAuthorizedKey(pub))
	if comment != "" {
		line = line[:len(line)-1] + " " + comment + "\n"
	}
	return line
}

// uniquePath returns path if it does not exist, otherwise path_1, path_2, …
func uniquePath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	base := path
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s_%d", base, i)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

func marshalED25519Private(key ed25519.PrivateKey) ([]byte, error) {
	b, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: b}), nil
}
