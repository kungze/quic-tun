package token

import (
	"bufio"
	"errors"
	"os"
	"strings"
)

type fileTokenSourcePlugin struct {
	filePath string
}

func (t fileTokenSourcePlugin) GetToken(addr string) (string, error) {
	ipAddr := strings.Split(addr, ":")[0]
	file, err := os.Open(t.filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, ipAddr) {
			return strings.Split(line, " ")[1], nil
		}
	}
	return "", errors.New("don't find valid token")
}

// NewFileTokenSourcePlugin return a ``File`` type token source plugin.
// ``File`` type token source plugin will read the token from a file.
// The tokenSource is the file path.
func NewFileTokenSourcePlugin(tokenSource string) fileTokenSourcePlugin {
	return fileTokenSourcePlugin{filePath: tokenSource}
}
