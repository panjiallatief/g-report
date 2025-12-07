package utils

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestHashAndCheckPassword(t *testing.T) {
	password := "rahasia123"

	// 1. Test Hash
	hash, err := HashPassword(password)
	assert.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, password, hash)

	// 2. Test Check (Correct)
	match := CheckPasswordHash(password, hash)
	assert.True(t, match, "Password harus cocok dengan hash")

	// 3. Test Check (Wrong)
	matchWrong := CheckPasswordHash("salah123", hash)
	assert.False(t, matchWrong, "Password salah tidak boleh cocok")
}
