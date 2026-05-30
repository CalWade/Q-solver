package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
)

// 固定密钥：由常量字符串 SHA256 得到 32 字节，用于 AES-256-GCM
var aesKey []byte

func init() {
	h := sha256.Sum256([]byte("JTwZwHaMq3g55PfVgxkah1Edst59LoyF"))
	aesKey = h[:]
}

func encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("密文过短")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// Cipher 暴露给同应用内其他持久化模块（如 history）共用密钥。
// 实现 history.Encryptor 接口，但本包不引用 history 以避免循环依赖。
type Cipher struct{}

// SharedCipher 是共享 cipher 的单例，导入方直接 .Encrypt/.Decrypt 即可。
var SharedCipher = Cipher{}

// Encrypt 调用同包的私有 encrypt（AES-256-GCM）。
func (Cipher) Encrypt(plaintext []byte) ([]byte, error) { return encrypt(plaintext) }

// Decrypt 调用同包的私有 decrypt。
func (Cipher) Decrypt(ciphertext []byte) ([]byte, error) { return decrypt(ciphertext) }
