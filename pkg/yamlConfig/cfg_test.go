package yamlConfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	// Create temporary test config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test_config.yaml")

	validConfig := `
service:
  name: "test-service"
  purpose: "testing"
  port: 8080
sources:
  - name: "test-db"
    type: "mysql"
    enabled: true
    location: "localhost:3306"
    auth:
      username: "test"
      password: "test123"
  - name: "api"
    type: "rest"
    enabled: true
    location: "http://api.example.com"
logging:
  level: "info"
  filePath: "/var/log"
  maxSize: 100
`

	err := os.WriteFile(configPath, []byte(validConfig), 0o644)
	require.NoError(t, err)

	t.Run("valid config loads successfully", func(t *testing.T) {
		err := Load(configPath)
		require.NoError(t, err)

		config := Get()
		assert.Equal(t, "test-service", config.Service().Name)
		assert.Equal(t, 2, len(config.Sources()))
	})

	t.Run("invalid file path returns error", func(t *testing.T) {
		err := Load("nonexistent.yaml")
		assert.Error(t, err)
	})

	t.Run("invalid yaml returns error", func(t *testing.T) {
		invalidPath := filepath.Join(tmpDir, "invalid.yaml")
		err := os.WriteFile(invalidPath, []byte("invalid: ][yaml"), 0o644)
		require.NoError(t, err)

		err = Load(invalidPath)
		assert.Error(t, err)
	})
}
