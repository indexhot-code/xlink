// Package vault 负责凭据加密：scrypt 从用户主口令派生主密钥 + AES-256-GCM 加密。
//
// 硬约束（架构规则 4）：密码与私钥口令在 SQLite 中必须以 AES-256-GCM 加密存储；
// 绝不明文落盘，不写入 Windows 凭据管理器；主密钥仅驻留内存，锁定/退出即丢弃。
package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"

	"golang.org/x/crypto/scrypt"
)

// KDF 参数：scrypt N=2^15（32 MiB 内存）、r=8、p=1。
// 参数随 salt 一并持久化，将来升级算法时可按记录迁移。
const (
	scryptN = 1 << 15
	scryptR = 8
	scryptP = 1
	keyLen  = 32
)

// ErrLocked 表示保险库未解锁（或未设置主口令）。
var ErrLocked = errors.New("vault is locked")

// ErrAuthFailed 表示口令错误。
var ErrAuthFailed = errors.New("invalid master password")

// KDFParams 是持久化的派生参数。
type KDFParams struct {
	N    int    `json:"n"`
	R    int    `json:"r"`
	P    int    `json:"p"`
	Salt []byte `json:"salt"`
}

// DefaultKDFParams 生成新的随机 salt 与默认参数。
func DefaultKDFParams() (KDFParams, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return KDFParams{}, fmt.Errorf("generate salt: %w", err)
	}
	return KDFParams{N: scryptN, R: scryptR, P: scryptP, Salt: salt}, nil
}

// DeriveMasterKey 从口令派生 256 位主密钥。
func DeriveMasterKey(password string, p KDFParams) ([]byte, error) {
	key, err := scrypt.Key([]byte(password), p.Salt, p.N, p.R, p.P, keyLen)
	if err != nil {
		return nil, fmt.Errorf("scrypt: %w", err)
	}
	return key, nil
}

// Encrypt 用主密钥做 AES-256-GCM 加密，返回 nonce||ciphertext。
func Encrypt(key, plaintext []byte) ([]byte, error) {
	if len(key) != keyLen {
		return nil, fmt.Errorf("master key must be %d bytes", keyLen)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt 解密 nonce||ciphertext。密钥错误或数据被篡改返回错误。
func Decrypt(key, blob []byte) ([]byte, error) {
	if len(key) != keyLen {
		return nil, fmt.Errorf("master key must be %d bytes", keyLen)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(blob) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ct := blob[:gcm.NonceSize()], blob[gcm.NonceSize():]
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return pt, nil
}

// verifierPlaintext 是口令校验用的已知明文。
var verifierPlaintext = []byte("xlink-vault-verifier-v1")

// MakeVerifier 用主密钥加密已知明文，存入 vault_meta 用于 Unlock 校验。
func MakeVerifier(key []byte) ([]byte, error) {
	return Encrypt(key, verifierPlaintext)
}

// CheckVerifier 尝试解密校验 blob，判定口令是否正确。
func CheckVerifier(key, blob []byte) error {
	pt, err := Decrypt(key, blob)
	if err != nil {
		return ErrAuthFailed
	}
	if subtle.ConstantTimeCompare(pt, verifierPlaintext) != 1 {
		return ErrAuthFailed
	}
	return nil
}
