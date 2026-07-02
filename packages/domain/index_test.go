package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewIDReturnsUUIDLikeValue(t *testing.T) {
	id := NewID()

	require.Len(t, id, 36)
	assert.Equal(t, byte('-'), id[8])
	assert.Equal(t, byte('-'), id[13])
	assert.Equal(t, byte('-'), id[18])
	assert.Equal(t, byte('-'), id[23])
	assert.Equal(t, byte('4'), id[14])
}

func TestPayloadMarshalsValuesAndFallsBackOnInvalidInput(t *testing.T) {
	assert.JSONEq(t, `{"name":"Farros"}`, string(Payload(map[string]string{"name": "Farros"})))
	assert.JSONEq(t, `{}`, string(Payload(func() {})))
}
