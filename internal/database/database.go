package database

import (
	"database/sql"
	"os"
	"path/filepath"
)

func NewConnection() (*sql.DB, error) {

	path, err := databasePath()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(path); err != nil {
		return nil, err
	}

	return sql.Open("sqlite", path)

}

func databasePath() (string, error) {
	own, err := os.Executable()
	if err != nil {
		return "", err
	}
	var path = filepath.Join(filepath.Dir(own), "..", "StreamingAssets", "patching.sqlite")
	return filepath.Clean(path), nil
}

func ExistsAtPath() bool {
	path, err := databasePath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	if err != nil {
		return false
	}
	return true
}
