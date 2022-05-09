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
	return "", errors.New("Don't find valid token.")
}

func NewFileTokenSourcePlugin(tokenSource string) fileTokenSourcePlugin {
	return fileTokenSourcePlugin{filePath: tokenSource}
}
