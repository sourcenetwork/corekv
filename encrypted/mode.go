package encrypted

import "crypto/cipher"

// Mode implements a specific encryption and decryption scheme
type Mode interface {
	// Encrypt returns the ciphertext for the given plaintext
	Encrypt(plaintext []byte) []byte
	// Decrypt returns the plaintext for the given ciphertext
	Decrypt(ciphertext []byte) []byte
}

var _ Mode = (*CFBMode)(nil)

// CFBMode encrypts or decrypts using cipher feedback mode
type CFBMode struct {
	block cipher.Block
	iv    []byte
}

// WithCFBMode returns a cipher feedback mode encryption scheme
func WithCFBMode(block cipher.Block, iv []byte) *CFBMode {
	return &CFBMode{block, iv}
}

func (m *CFBMode) Encrypt(plaintext []byte) []byte {
	ciphertext := make([]byte, len(plaintext))
	cipher.NewCFBEncrypter(m.block, m.iv).XORKeyStream(ciphertext, plaintext)
	return ciphertext
}

func (m *CFBMode) Decrypt(ciphertext []byte) []byte {
	plaintext := make([]byte, len(ciphertext))
	cipher.NewCFBDecrypter(m.block, m.iv).XORKeyStream(plaintext, ciphertext)
	return plaintext
}

var _ Mode = (*CTRMode)(nil)

// CTRMode encrypts or decrypts using counter mode
type CTRMode struct {
	block cipher.Block
	iv    []byte
}

// WithCTRMode returns a counter mode encryption scheme
func WithCTRMode(block cipher.Block, iv []byte) *CTRMode {
	return &CTRMode{block, iv}
}

func (m *CTRMode) Encrypt(plaintext []byte) []byte {
	ciphertext := make([]byte, len(plaintext))
	cipher.NewCTR(m.block, m.iv).XORKeyStream(plaintext, ciphertext)
	return ciphertext
}

func (m *CTRMode) Decrypt(ciphertext []byte) []byte {
	plaintext := make([]byte, len(ciphertext))
	cipher.NewCTR(m.block, m.iv).XORKeyStream(plaintext, ciphertext)
	return plaintext
}

var _ Mode = (*OFBMode)(nil)

// OFBMode encrypts or decrypts using output feedback mode
type OFBMode struct {
	block cipher.Block
	iv    []byte
}

// WithCTRMode returns an output feedback mode encryption scheme
func WithOFBMode(block cipher.Block, iv []byte) *OFBMode {
	return &OFBMode{block, iv}
}

func (m *OFBMode) Encrypt(plaintext []byte) []byte {
	ciphertext := make([]byte, len(plaintext))
	cipher.NewOFB(m.block, m.iv).XORKeyStream(plaintext, ciphertext)
	return ciphertext
}

func (m *OFBMode) Decrypt(ciphertext []byte) []byte {
	plaintext := make([]byte, len(ciphertext))
	cipher.NewOFB(m.block, m.iv).XORKeyStream(plaintext, ciphertext)
	return plaintext
}
