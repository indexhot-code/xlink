package connection

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"
)

// parseKeyFile 读取并解析私钥文件，支持口令保护密钥。
func parseKeyFile(path, passphrase string) (ssh.Signer, error) {
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read key file: %w", err)
	}
	if passphrase == "" {
		signer, err := ssh.ParsePrivateKey(pem)
		if err != nil {
			var perr *ssh.PassphraseMissingError
			if errors.As(err, &perr) {
				return nil, errors.New("key requires a passphrase")
			}
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		return signer, nil
	}
	signer, err := ssh.ParsePrivateKeyWithPassphrase(pem, []byte(passphrase))
	if err != nil {
		return nil, fmt.Errorf("parse private key (with passphrase): %w", err)
	}
	return signer, nil
}
