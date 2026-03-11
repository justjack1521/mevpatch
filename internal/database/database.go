package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
)

func NewConnection(app string) (*sql.DB, error) {

	path, err := databasePath(app)
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(path); err != nil {
		return nil, err
	}

	dbc, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	_, err = dbc.Exec("PRAGMA busy_timeout = 5000;")
	if err != nil {
		return nil, err
	}
	_, err = dbc.Exec("PRAGMA journal_mode = WAL;")
	if err != nil {
		return nil, err
	}
	return dbc, nil

}

func databasePath(app string) (string, error) {
	var low = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "LocalLow")
	var path = filepath.Join(low, "BlankProjectDev", "Blank Project Launcher", fmt.Sprintf("%s.sqlite", "patching"))
	return filepath.Clean(path), nil
}

func ExistsAtPath(app string) bool {
	path, err := databasePath(app)
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	if err != nil {
		return false
	}
	return true
}
