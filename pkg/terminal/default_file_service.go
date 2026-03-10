package terminal

import (
	"os"
)

// DefaultFileService provides default file system operations using the os package
type DefaultFileService struct{}

// NewDefaultFileService creates a new DefaultFileService
func NewDefaultFileService() *DefaultFileService {
	return &DefaultFileService{}
}

// WriteFile writes data to a file, using os.WriteFile. perm is optional.
func (f *DefaultFileService) WriteFile(filename string, data []byte, perm ...interface{}) error {
	var filePerm os.FileMode = 0644
	if len(perm) > 0 {
		if p, ok := perm[0].(os.FileMode); ok {
			filePerm = p
		} else if p, ok := perm[0].(int); ok {
			filePerm = os.FileMode(p)
		}
	}
	return os.WriteFile(filename, data, filePerm)
}

// MkdirAll creates a directory path, using os.MkdirAll. perm is optional.
func (f *DefaultFileService) MkdirAll(path string, perm ...interface{}) error {
	var dirPerm os.FileMode = 0755
	if len(perm) > 0 {
		if p, ok := perm[0].(os.FileMode); ok {
			dirPerm = p
		} else if p, ok := perm[0].(int); ok {
			dirPerm = os.FileMode(p)
		}
	}
	return os.MkdirAll(path, dirPerm)
}
