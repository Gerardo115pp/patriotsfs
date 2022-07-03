package patriotsfs

import (
	"net/http"
	"os"
)

const KB int = 1024
const MB int = 1024 * KB
const GB int = 1024 * MB
const TB int = 1024 * GB

type Middleware func(http.HandlerFunc) http.HandlerFunc

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
