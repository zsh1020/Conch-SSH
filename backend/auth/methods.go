package auth

import (
	"fmt"
	"net"
	"os"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// PasswordPromptFunc is called when a password is needed interactively.
// For GUI use, this will be wired to a frontend dialog.
type PasswordPromptFunc func(host, user string) (string, error)

// BuildMethods constructs the ssh.AuthMethod list based on authType.
// passwordOrPassphrase is used directly (already collected by the UI).
func BuildMethods(authType, identityFile, password string) ([]ssh.AuthMethod, error) {
	switch authType {
	case "password":
		return []ssh.AuthMethod{ssh.Password(password)}, nil

	case "key":
		return buildKeyAuth(identityFile, password)

	case "agent":
		return buildAgentAuth()

	default:
		// Try agent first, then key, then password
		var methods []ssh.AuthMethod
		if am, err := buildAgentAuth(); err == nil {
			methods = append(methods, am...)
		}
		if identityFile != "" {
			if km, err := buildKeyAuth(identityFile, password); err == nil {
				methods = append(methods, km...)
			}
		}
		if password != "" {
			methods = append(methods, ssh.Password(password))
		}
		if len(methods) == 0 {
			return nil, fmt.Errorf("no authentication method available")
		}
		return methods, nil
	}
}

func buildKeyAuth(identityFile, passphrase string) ([]ssh.AuthMethod, error) {
	path := identityFile
	if len(path) > 1 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		path = home + path[1:]
	}

	keyData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read identity file: %w", err)
	}

	var signer ssh.Signer
	if passphrase != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(passphrase))
	} else {
		signer, err = ssh.ParsePrivateKey(keyData)
	}
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
}

func buildAgentAuth() ([]ssh.AuthMethod, error) {
	sockPath := os.Getenv("SSH_AUTH_SOCK")
	if sockPath == "" {
		return nil, fmt.Errorf("SSH_AUTH_SOCK not set")
	}
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		return nil, fmt.Errorf("connect to ssh-agent: %w", err)
	}
	ag := agent.NewClient(conn)
	return []ssh.AuthMethod{ssh.PublicKeysCallback(ag.Signers)}, nil
}
