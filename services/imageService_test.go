package services

import (
	"crypto/rand"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_SetAccountCoverImageLarge(t *testing.T) {
	connectToDb()

	bytes := make([]byte, 1024*1024)
	_, err := rand.Read(bytes)
	require.Nil(t, err)

	imgBase64 := base64.RawStdEncoding.EncodeToString(bytes)
	err = SetAccountCoverImage(1, &imgBase64)
	require.Nil(t, err)
}

func Test_SetAccountCoverImageTooLarge(t *testing.T) {
	connectToDb()

	bytes := make([]byte, 1024*1024+1)
	_, err := rand.Read(bytes)
	require.Nil(t, err)

	imgBase64 := base64.RawStdEncoding.EncodeToString(bytes)
	err = SetAccountCoverImage(1, &imgBase64)
	require.NotNil(t, err)
}
