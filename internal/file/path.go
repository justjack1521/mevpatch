package file

import (
	"fmt"
	"os"
	"path/filepath"
)

// PatcherDir returns the directory containing the patcher executable.
func PatcherDir() (string, error) {
	own, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolving executable path: %w", err)
	}
	return filepath.Dir(own), nil
}

// PersistentPath resolves a normalised file path to its location on disk.
//
// Directory layout:
//
//	game_root/                                  ← launcher files live here
//	game_root/Game/                             ← game files live here
//	game_root/Blank Project Launcher_Data/
//	  Patching/                                 ← patcher.exe and .version files live here
//
// So relative to patcher.exe:
//
//	launcher files → ../../{normal}             (up from Patching/, up from Launcher_Data/)
//	game files     → ../../Game/{normal}
func PersistentPath(app string, normal string) (string, error) {
	patcher, err := PatcherDir()
	if err != nil {
		return "", err
	}
	var path string
	switch app {
	case "launcher":
		path = filepath.Join(patcher, "..", "..", normal)
	default:
		// All other apps (including "game") live in a subdirectory of game_root
		// whose name matches the app name with correct casing.
		// We use "Game" as the canonical folder name for "game".
		path = filepath.Join(patcher, "..", "..", "Game", normal)
	}
	return filepath.Clean(path), nil
}

// VersionFilePath returns the path to an app's version file.
// Stored as game_root/Patching/{app}.version, right next to the patcher exe.
func VersionFilePath(app string) (string, error) {
	patcher, err := PatcherDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(patcher, fmt.Sprintf("%s.version", app)), nil
}

// LauncherExePath returns the full path to the launcher executable.
func LauncherExePath() (string, error) {
	return PersistentPath("launcher", "Blank Project Launcher.exe")
}
func TemporaryPath(app string, normal string) string {
	return filepath.Clean(filepath.Join(os.TempDir(), "mevpatch", app, normal))
}

// PatchBundlePath returns the temp path for a downloaded patch bundle zip.
func PatchBundlePath(app string, version string) string {
	return TemporaryPath(app, filepath.Join("bundles", fmt.Sprintf("%s_patch.bin", version)))
}

// ExtractPath returns the temp path for extracted patch files.
func ExtractPath(app string, name string) string {
	return TemporaryPath(app, filepath.Join("extract", name))
}
