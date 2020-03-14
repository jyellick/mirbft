package network

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	config, err := LoadConfig("config.yaml")
	assert.NoError(t, err)
	_, err = NewID(config)
	assert.NoError(t, err)
}
