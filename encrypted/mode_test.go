package encrypted

import (
	"crypto/aes"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ModeTestSuite struct {
	suite.Suite
	mode Mode
}

func (suite *ModeTestSuite) TestEncryptThenDecrypt() {
	input := []byte("hello world")

	ciphertext := suite.mode.Encrypt(input)
	suite.Assert().NotEqual(input, ciphertext)

	plaintext := suite.mode.Decrypt(ciphertext)
	suite.Assert().Equal(input, plaintext)
}

func TestCFBMode(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)

	block, err := aes.NewCipher(key)
	require.NoError(t, err)

	iv := make([]byte, block.BlockSize())
	_, err = rand.Read(key)
	require.NoError(t, err)

	mode := WithCFBMode(block, iv)
	suite.Run(t, &ModeTestSuite{mode: mode})
}

func TestCTRMode(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)

	block, err := aes.NewCipher(key)
	require.NoError(t, err)

	iv := make([]byte, block.BlockSize())
	_, err = rand.Read(key)
	require.NoError(t, err)

	mode := WithCTRMode(block, iv)
	suite.Run(t, &ModeTestSuite{mode: mode})
}

func TestOFBMode(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)

	block, err := aes.NewCipher(key)
	require.NoError(t, err)

	iv := make([]byte, block.BlockSize())
	_, err = rand.Read(key)
	require.NoError(t, err)

	mode := WithOFBMode(block, iv)
	suite.Run(t, &ModeTestSuite{mode: mode})
}
