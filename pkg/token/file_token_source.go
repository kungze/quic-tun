package token

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type fileTokenSourcePlugin struct {
	filePath string
}

func (t fileTokenSourcePlugin) GetToken(ipOrPort string) (string, error) {
	file, err := os.Open(t.filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file %s: %w", t.filePath, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		items := strings.Split(line, " ")
		if len(items) != 2 {
			continue
		}
		addr := items[0]
		token := items[1]
		if addr == ipOrPort {
			return token, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to scan file %s: %w", t.filePath, err)
	}

	return "", fmt.Errorf("cannot find token for %s in file %s", ipOrPort, t.filePath)
}

// NewFileTokenSourcePlugin return a ``File`` type token source plugin.
// ``File`` type token source plugin will read the token from a file.
// The tokenSource is the file path.
func NewFileTokenSourcePlugin(tokenSource string) fileTokenSourcePlugin {
	return fileTokenSourcePlugin{filePath: tokenSource}
}
