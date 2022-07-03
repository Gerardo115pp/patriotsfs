package patriotsfs

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
)

// TODO: Add recursive capabilitys, right now it only works with one level and the directorys are ignored

type PatriotsFsDirectory struct {
	BasePath string
	BaseName string
	content  map[string]fs.FileInfo
}

func (self *PatriotsFsDirectory) Exists(path string) bool {
	_, exists := self.content[path]
	if exists != pathExists(path) {
		self.verifyContentIntegrity()
	}
	return exists
}

func (self *PatriotsFsDirectory) GetFileAbsoultPath(filename string) string {
	return fmt.Sprintf("%s/%s", self.BasePath, filename)
}

func (self *PatriotsFsDirectory) getFile(file_name string) (fs.FileInfo, error) {
	return os.Stat(fmt.Sprintf("%s/%s", self.BasePath, file_name))
}

func (self *PatriotsFsDirectory) IsEmpty() bool {
	return len(self.content) == 0
}

func (self *PatriotsFsDirectory) rebase(new_base string) error {
	if new_base != self.BasePath && pathExists(new_base) {
		self.BasePath = new_base
		self.content = make(map[string]fs.FileInfo)
		self.verifyContentIntegrity()
		return nil
	} else {
		return fmt.Errorf("%s does not exist", new_base)
	}
}

func (self *PatriotsFsDirectory) verifyContentIntegrity() {
	directory_files, err := ioutil.ReadDir(self.BasePath)
	if err != nil {
		fmt.Println(err)
	}

	var found int = 0
	for _, file := range directory_files {
		if file.IsDir() {
			continue
		}
		self.content[file.Name()] = file
		if _, exists := self.content[file.Name()]; exists {
			found++
		}
	}
	if found != len(self.content) {
		for filename, file := range self.content {
			if file.IsDir() {
				continue
			}
			if !pathExists(self.GetFileAbsoultPath(filename)) {
				delete(self.content, filename)
			}
		}
	}
}

func (self *PatriotsFsDirectory) WriteFile(file_name string, content []byte, permissions fs.FileMode) error {
	filepath := fmt.Sprintf("%s/%s", self.BasePath, file_name)
	err := ioutil.WriteFile(filepath, content, permissions)
	if err != nil {
		return err
	}
	self.verifyContentIntegrity()
	return nil
}

func CreateNewFsDirectory(base string) (*PatriotsFsDirectory, error) {
	var fsDirectory *PatriotsFsDirectory = new(PatriotsFsDirectory)
	fsDirectory.BasePath = base
	fsDirectory.BaseName = filepath.Base(base)
	fsDirectory.content = make(map[string]fs.FileInfo)
	fsDirectory.verifyContentIntegrity()
	return fsDirectory, nil
}

func setDirectory(base string) (*PatriotsFsDirectory, error) {
	if pathExists(base) && isDir(base) {
		return CreateNewFsDirectory(base)
	} else {
		return nil, fmt.Errorf("%s does not exist", base)
	}
}
