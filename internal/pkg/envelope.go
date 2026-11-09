// Package pkg holds small utilities shared by the CLI and handlers: envelope
// encryption, random tokens, and Argon2id password hashing.
package pkg

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"fmt"
)

const envelopeVersion byte = 1

// Keyring holds AES-256 master keys by ID. Every ciphertext records the key
// that produced it so a rotation can re-encrypt rows without downtime.
type Keyring struct {
	keys   map[string][]byte
	active string
}

// NewKeyring validates the key sizes and that the active ID exists.
func NewKeyring(keys map[string][]byte, activeID string) (*Keyring, error) {
	if len(keys) == 0 {
		return nil, errors.New("keyring is empty")
	}
	copied := make(map[string][]byte, len(keys))
	for id, key := range keys {
		if id == "" {
			return nil, errors.New("key id must not be empty")
		}
		if len(key) != 32 {
			return nil, fmt.Errorf("key %q must be 32 bytes, got %d", id, len(key))
		}
		copied[id] = append([]byte(nil), key...)
	}
	if _, ok := copied[activeID]; !ok {
		return nil, fmt.Errorf("active key %q is not in the keyring", activeID)
	}
	return &Keyring{keys: copied, active: activeID}, nil
}

// ActiveKeyID returns the key used for new ciphertext.
func (k *Keyring) ActiveKeyID() string { return k.active }

// Encrypt seals plaintext with the active key.
func (k *Keyring) Encrypt(plaintext []byte) ([]byte, error) {
	return k.EncryptWith(plaintext, k.active)
}

// EncryptWith seals plaintext with a specific key. It exists for re-encryption
// tooling and tests.
func (k *Keyring) EncryptWith(plaintext []byte, keyID string) ([]byte, error) {
	key, ok := k.keys[keyID]
	if !ok {
		return nil, fmt.Errorf("unknown key id %q", keyID)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	if len(keyID) > 255 {
		return nil, errors.New("key id is too long")
	}

	envelope := make([]byte, 0, 2+len(keyID)+aead.NonceSize()+len(plaintext)+aead.Overhead())
	envelope = append(envelope, envelopeVersion, byte(len(keyID)))
	envelope = append(envelope, keyID...)
	nonce := make([]byte, aead.NonceSize())
	if _, err := randomRead(nonce); err != nil {
		return nil, err
	}
	envelope = append(envelope, nonce...)
	header := envelope[:2+len(keyID)]
	return aead.Seal(envelope, nonce, plaintext, header), nil
}

// Decrypt opens an envelope with the key it names.
func (k *Keyring) Decrypt(envelope []byte) ([]byte, error) {
	if len(envelope) < 2 {
		return nil, errors.New("envelope is truncated")
	}
	if envelope[0] != envelopeVersion {
		return nil, fmt.Errorf("unsupported envelope version %d", envelope[0])
	}
	kidLen := int(envelope[1])
	headerLen := 2 + kidLen
	if len(envelope) < headerLen {
		return nil, errors.New("envelope key id is truncated")
	}
	keyID := string(envelope[2:headerLen])
	key, ok := k.keys[keyID]
	if !ok {
		return nil, fmt.Errorf("unknown key id %q", keyID)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	if len(envelope) < headerLen+aead.NonceSize()+aead.Overhead() {
		return nil, errors.New("envelope ciphertext is truncated")
	}
	nonce := envelope[headerLen : headerLen+aead.NonceSize()]
	ciphertext := envelope[headerLen+aead.NonceSize():]
	plaintext, err := aead.Open(nil, nonce, ciphertext, envelope[:headerLen])
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return plaintext, nil
}

// KeyID reports which key encrypted an envelope without decrypting it.
func (k *Keyring) KeyID(envelope []byte) (string, error) {
	if len(envelope) < 2 {
		return "", errors.New("envelope is truncated")
	}
	kidLen := int(envelope[1])
	if len(envelope) < 2+kidLen {
		return "", errors.New("envelope key id is truncated")
	}
	return string(envelope[2 : 2+kidLen]), nil
}
