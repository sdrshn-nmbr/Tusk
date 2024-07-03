package storage

import (
	"io"
	"os"
	"path/filepath"
)

type FileStorage struct {
	RootDir string
}

func NewFileStorage(rootDir string) (*FileStorage, error) {
	// * permission bits set to 0755 -> owners: r, w, e & others: r, e
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		return nil, err
	}
	storage := &FileStorage{
		RootDir: rootDir,
	}

	return storage, nil

}

func (fs *FileStorage) SaveFile(filename string, content io.Reader) error {
	path := filepath.Join(fs.RootDir, filename)
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, content)
	return err
}

func (fs *FileStorage) GetFile(filename string) ([]byte, error) {
	path := filepath.Join(fs.RootDir, filename)
	return os.ReadFile(path)
}

func (fs *FileStorage) DeleteFileFunc(filename string) error {
	path := filepath.Join(fs.RootDir, filename)
	return os.Remove(path)
}

func (fs *FileStorage) ListFiles() ([]string, error) {
	var files []string
	err := filepath.Walk(
		fs.RootDir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				relPath, _ := filepath.Rel(fs.RootDir, path)
				files = append(files, relPath)
			}

			return nil
		})

	return files, err
}

func (fs *FileStorage) GetFileSize(filename string) (int64, error) {
	path := filepath.Join(fs.RootDir, filename)
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}
