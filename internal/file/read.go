package file

import (
	"errors"
	"os"
)

func SizeAtPath(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func CanReadAtPath(path string) error {
	if err := ExistsAtPath(path); err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	file.Close()
	return nil
}

func ExistsAtPath(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Size() == 0 {
		return errors.New("file exists but is empty")
	}
	return nil
}
