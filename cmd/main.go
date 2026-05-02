package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/justjack1521/mevpatch/internal/file"
	"github.com/justjack1521/mevpatch/internal/gui"
	"github.com/justjack1521/mevpatch/internal/manifest"
	"github.com/justjack1521/mevpatch/internal/orchestrate"
	"github.com/justjack1521/mevpatch/internal/patch"
	"github.com/spf13/cobra"
)

//go:embed bin/hpatchz.exe
var mergerBytes []byte

var rootCmd = &cobra.Command{
	Use:   "mevpatch",
	Short: "Mevius patcher",
}

var patchCmd = &cobra.Command{
	Use:   "patch",
	Short: "Patch an application to a target version",
	RunE:  runPatch,
}

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify all installed files match the current version",
	RunE:  runVerify,
}

var repairCmd = &cobra.Command{
	Use:   "repair",
	Short: "Force re-download and re-patch all files, ignoring local state",
	RunE:  runRepair,
}

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Remove files not listed in the installed version manifest",
	RunE:  runCleanup,
}

var (
	flagApp           string
	flagVersion       string
	flagDebug         bool
	flagVerifyApp     string
	flagRepairApp     string
	flagRepairVersion string
	flagCleanupApp    string
	flagHTTP1         bool
)

func init() {
	patchCmd.Flags().StringVarP(&flagApp, "app", "a", "", "application name (launcher or game)")
	patchCmd.Flags().StringVarP(&flagVersion, "version", "v", "", "target version (e.g. 1.2.3)")
	patchCmd.Flags().BoolVarP(&flagDebug, "debug", "d", false, "enable debug logging")
	patchCmd.Flags().BoolVar(&flagHTTP1, "http1", false, "force HTTP/1.1 (workaround for HTTP/2 stream errors)")
	patchCmd.MarkFlagRequired("app")

	verifyCmd.Flags().StringVarP(&flagVerifyApp, "app", "a", "", "application name (launcher or game)")
	verifyCmd.MarkFlagRequired("app")

	repairCmd.Flags().StringVarP(&flagRepairApp, "app", "a", "", "application name (launcher or game)")
	repairCmd.Flags().StringVarP(&flagRepairVersion, "version", "v", "", "target version to repair to (e.g. 1.2.3)")
	repairCmd.Flags().BoolVarP(&flagDebug, "debug", "d", false, "enable debug logging")
	repairCmd.Flags().BoolVar(&flagHTTP1, "http1", false, "force HTTP/1.1 (workaround for HTTP/2 stream errors)")
	repairCmd.MarkFlagRequired("app")

	cleanupCmd.Flags().StringVarP(&flagCleanupApp, "app", "a", "", "application name (launcher or game)")
	cleanupCmd.MarkFlagRequired("app")

	rootCmd.AddCommand(patchCmd)
	rootCmd.AddCommand(verifyCmd)
	rootCmd.AddCommand(repairCmd)
	rootCmd.AddCommand(cleanupCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func relaunchLauncher(app string) {
	if app != "launcher" {
		return
	}
	path, err := file.LauncherExePath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "relaunch: could not resolve launcher path: %v\n", err)
		return
	}
	if err := exec.Command(path).Start(); err != nil {
		fmt.Fprintf(os.Stderr, "relaunch: failed to start launcher: %v\n", err)
	}
}

func runPatch(cmd *cobra.Command, args []string) error {

	time.Sleep(3 * time.Second)

	var (
		version patch.Version
		err     error
	)
	if flagVersion == "" || flagVersion == "latest" {
		version, err = manifest.FetchCurrentVersion(flagApp)
		if err != nil {
			return fmt.Errorf("failed to fetch current version: %w", err)
		}
	} else {
		version, err = patch.NewVersion(flagVersion)
		if err != nil {
			return fmt.Errorf("invalid version %q: %w", flagVersion, err)
		}
	}

	merger, err := patch.CreateMergeTool(mergerBytes)
	if err != nil {
		return fmt.Errorf("extracting merge tool: %w", err)
	}
	defer os.RemoveAll(merger.Dir)

	updates := make(chan gui.PatchUpdate, 32)

	o := orchestrate.NewOrchestrator(orchestrate.OrchestratorArguments{
		Application:          flagApp,
		TargetVersion:        version,
		LegacyConnectionMode: flagHTTP1,
		Debug:                flagDebug,
	}, merger, updates, []orchestrate.OrchestratorStep{
		orchestrate.NewVersionCheckStep(),
		orchestrate.NewRebuildStateStep(),
		orchestrate.NewManifestDownloadStep(),
		orchestrate.NewRepairStep(),
	})

	go o.Start(context.Background())

	gui.NewWindow(flagApp, version, updates).Build()

	relaunchLauncher(flagApp)
	return nil
}

func runVerify(cmd *cobra.Command, args []string) error {

	time.Sleep(3 * time.Second)

	merger, err := patch.CreateMergeTool(mergerBytes)
	if err != nil {
		return fmt.Errorf("extracting merge tool: %w", err)
	}
	defer os.RemoveAll(merger.Dir)

	updates := make(chan gui.PatchUpdate, 32)

	o := orchestrate.NewOrchestrator(orchestrate.OrchestratorArguments{
		Application:   flagVerifyApp,
		TargetVersion: patch.Version{},
	}, merger, updates, []orchestrate.OrchestratorStep{
		orchestrate.NewVersionCheckStep(),
		orchestrate.NewRebuildStateStep(),
		orchestrate.NewVerifyStep(),
	})

	go o.Start(context.Background())

	gui.NewWindow(flagVerifyApp, patch.Version{}, updates).Build()

	relaunchLauncher(flagVerifyApp)
	return nil
}

func runRepair(cmd *cobra.Command, args []string) error {

	time.Sleep(3 * time.Second)

	var (
		version patch.Version
		err     error
	)
	if flagRepairVersion == "" || flagRepairVersion == "latest" {
		version, err = manifest.FetchCurrentVersion(flagApp)
		if err != nil {
			return fmt.Errorf("failed to fetch current version: %w", err)
		}
	} else {
		version, err = patch.NewVersion(flagVersion)
		if err != nil {
			return fmt.Errorf("invalid version %q: %w", flagRepairVersion, err)
		}
	}

	merger, err := patch.CreateMergeTool(mergerBytes)
	if err != nil {
		return fmt.Errorf("extracting merge tool: %w", err)
	}
	defer os.RemoveAll(merger.Dir)

	updates := make(chan gui.PatchUpdate, 32)

	o := orchestrate.NewOrchestrator(orchestrate.OrchestratorArguments{
		Application:          flagRepairApp,
		TargetVersion:        version,
		LegacyConnectionMode: flagHTTP1,
		Debug:                flagDebug,
	}, merger, updates, []orchestrate.OrchestratorStep{
		orchestrate.NewVersionCheckStep(),
		orchestrate.NewRebuildStateStep(),
		orchestrate.NewManifestDownloadStep(),
		orchestrate.NewRepairStep(),
	})

	go o.Start(context.Background())

	gui.NewWindow(flagRepairApp, version, updates).Build()

	relaunchLauncher(flagRepairApp)
	return nil
}

func runCleanup(cmd *cobra.Command, args []string) error {

	time.Sleep(3 * time.Second)

	merger, err := patch.CreateMergeTool(mergerBytes)
	if err != nil {
		return fmt.Errorf("extracting merge tool: %w", err)
	}
	defer os.RemoveAll(merger.Dir)

	updates := make(chan gui.PatchUpdate, 32)

	o := orchestrate.NewOrchestrator(orchestrate.OrchestratorArguments{
		Application:   flagCleanupApp,
		TargetVersion: patch.Version{},
	}, merger, updates, []orchestrate.OrchestratorStep{
		orchestrate.NewVersionCheckStep(),
		orchestrate.NewRebuildStateStep(),
		orchestrate.NewCleanupStep(),
	})

	go o.Start(context.Background())

	gui.NewWindow(flagCleanupApp, patch.Version{}, updates).Build()

	relaunchLauncher(flagCleanupApp)
	return nil
}
