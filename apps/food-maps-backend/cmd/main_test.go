package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOptions(t *testing.T) {
	opts := Options{
		Port:   8080,
		DBPath: "test.db",
	}

	assert.Equal(t, 8080, opts.Port)
	assert.Equal(t, "test.db", opts.DBPath)
}
