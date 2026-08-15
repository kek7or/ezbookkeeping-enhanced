package settings

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

const environmentFileName = ".env"

// loadEnvironmentFile reads the ".env" file in the working path and puts what it declares into the
// process environment, so that "EBK_CRYPTO_COINSTATS_API_KEY=..." in that file has the same effect
// as exporting it in the shell
//
// A variable that is already set in the real environment is left alone. The file supplies defaults
// for a local run; it never overrides what a service manager or a container was told to use.
//
// The file is optional and an unreadable one is not fatal: a server that starts with the settings
// from its configuration file is better than a server that refuses to start at all.
func loadEnvironmentFile(workingPath string) {
	file, err := os.Open(filepath.Join(workingPath, environmentFileName))

	if err != nil {
		return
	}

	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		name, value, ok := parseEnvironmentFileLine(scanner.Text())

		if !ok {
			continue
		}

		if _, exists := os.LookupEnv(name); exists {
			continue
		}

		os.Setenv(name, value)
	}
}

// parseEnvironmentFileLine returns the variable one line of an environment file declares, and
// whether the line declares one at all
func parseEnvironmentFileLine(line string) (string, string, bool) {
	line = strings.TrimSpace(line)

	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}

	line = strings.TrimPrefix(line, "export ")
	separatorIndex := strings.Index(line, "=")

	if separatorIndex < 1 {
		return "", "", false
	}

	name := strings.TrimSpace(line[:separatorIndex])
	value := strings.TrimSpace(line[separatorIndex+1:])

	if !isValidEnvironmentVariableName(name) {
		return "", "", false
	}

	return name, trimEnvironmentValueQuotes(value), true
}

// trimEnvironmentValueQuotes removes one matching pair of surrounding quotes, so that a value
// written with quotes to keep its spaces does not arrive with the quotes in it
func trimEnvironmentValueQuotes(value string) string {
	if len(value) < 2 {
		return value
	}

	firstChar := value[0]
	lastChar := value[len(value)-1]

	if (firstChar == '"' && lastChar == '"') || (firstChar == '\'' && lastChar == '\'') {
		return value[1 : len(value)-1]
	}

	return value
}

func isValidEnvironmentVariableName(name string) bool {
	if name == "" {
		return false
	}

	for i := 0; i < len(name); i++ {
		char := name[i]

		if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || char == '_' {
			continue
		}

		if char >= '0' && char <= '9' && i > 0 {
			continue
		}

		return false
	}

	return true
}
