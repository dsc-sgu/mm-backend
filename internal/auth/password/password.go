package password

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"runtime"

	"golang.org/x/crypto/argon2"
)

const (
	HASH_TIME   = 1
	HASH_MEMORY = 64 * 1024
	HASH_LEN    = 32
	SALT_LEN    = 16
)

func GenerateSalt() ([]byte, error) {
	salt := make([]byte, SALT_LEN)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}
	return salt, nil
}

func Hash(password string, salt []byte) []byte {
	hash := argon2.IDKey(
		[]byte(password),
		salt,
		HASH_TIME,
		HASH_MEMORY,
		(uint8)(runtime.NumCPU()),
		HASH_LEN,
	)
	return hash
}

func Valid(
	enteredPassword string,
	passwordHash []byte,
	passwordSalt []byte,
) bool {
	enteredPasswordHash := Hash(enteredPassword, passwordSalt)
	return bytes.Equal(enteredPasswordHash, passwordHash)
}
