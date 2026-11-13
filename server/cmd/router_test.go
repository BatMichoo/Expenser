package main

import (
	"testing"

	"expenser/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRouterMode(t *testing.T) {
	t.Cleanup(func() { gin.SetMode(gin.TestMode) })
	_, err := newRouter(&config.Config{})
	require.NoError(t, err)
	assert.Equal(t, gin.ReleaseMode, gin.Mode())

	_, err = newRouter(&config.Config{DevMode: true})
	require.NoError(t, err)
	assert.Equal(t, gin.DebugMode, gin.Mode())
}
