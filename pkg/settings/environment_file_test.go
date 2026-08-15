package settings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseEnvironmentFileLine(t *testing.T) {
	name, value, ok := parseEnvironmentFileLine("EBK_CRYPTO_COINSTATS_API_KEY=abc123")

	assert.True(t, ok)
	assert.Equal(t, "EBK_CRYPTO_COINSTATS_API_KEY", name)
	assert.Equal(t, "abc123", value)

	name, value, ok = parseEnvironmentFileLine("  EBK_SERVER_DOMAIN = example.com  ")

	assert.True(t, ok)
	assert.Equal(t, "EBK_SERVER_DOMAIN", name)
	assert.Equal(t, "example.com", value)

	name, value, ok = parseEnvironmentFileLine("export EBK_SECURITY_SECRET_KEY=\"a secret with spaces\"")

	assert.True(t, ok)
	assert.Equal(t, "EBK_SECURITY_SECRET_KEY", name)
	assert.Equal(t, "a secret with spaces", value)

	_, value, ok = parseEnvironmentFileLine("EBK_A='single quoted'")

	assert.True(t, ok)
	assert.Equal(t, "single quoted", value)

	// a value containing "=" keeps everything after the first one
	_, value, ok = parseEnvironmentFileLine("EBK_B=a=b=c")

	assert.True(t, ok)
	assert.Equal(t, "a=b=c", value)

	// an empty value is a declaration, not a malformed line
	_, value, ok = parseEnvironmentFileLine("EBK_C=")

	assert.True(t, ok)
	assert.Equal(t, "", value)
}

func TestParseEnvironmentFileLine_NotADeclaration(t *testing.T) {
	notDeclarations := []string{
		"",
		"   ",
		"# EBK_CRYPTO_COINSTATS_API_KEY=abc123",
		"EBK_CRYPTO_COINSTATS_API_KEY",
		"=abc123",
		"EBK-CRYPTO=abc123",
		"1EBK=abc123",
	}

	for i := 0; i < len(notDeclarations); i++ {
		_, _, ok := parseEnvironmentFileLine(notDeclarations[i])
		assert.False(t, ok, "line \"%s\" should not declare a variable", notDeclarations[i])
	}
}

func TestLoadEnvironmentFile(t *testing.T) {
	workingPath := t.TempDir()
	content := "# a comment\n\nEBK_TEST_FROM_FILE=from-file\nEBK_TEST_ALREADY_SET=from-file\n"

	err := os.WriteFile(filepath.Join(workingPath, environmentFileName), []byte(content), 0600)
	assert.Nil(t, err)

	t.Setenv("EBK_TEST_ALREADY_SET", "from-environment")

	loadEnvironmentFile(workingPath)

	assert.Equal(t, "from-file", os.Getenv("EBK_TEST_FROM_FILE"))

	// the real environment always wins, so a container or a service manager cannot be overruled
	// by a file that happens to sit in the working directory
	assert.Equal(t, "from-environment", os.Getenv("EBK_TEST_ALREADY_SET"))

	os.Unsetenv("EBK_TEST_FROM_FILE")
}

func TestLoadEnvironmentFile_NoFile(t *testing.T) {
	// a missing environment file is the normal case and must not panic or fail
	assert.NotPanics(t, func() {
		loadEnvironmentFile(t.TempDir())
	})
}
