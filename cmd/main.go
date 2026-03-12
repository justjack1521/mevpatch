package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"gioui.org/app"
	"github.com/justjack1521/mevpatch/internal/gui"
	"github.com/justjack1521/mevpatch/internal/orchestrate"
	"github.com/justjack1521/mevpatch/internal/patch"
	"os"
)

//go:embed bin/hpatchz.exe
var mergerBytes []byte

func main() {
	var a string
	var v string
	var d bool

	flag.StringVar(&a, "a", "", "application name (launcher or game)")
	flag.StringVar(&v, "v", "", "target version to patch to (e.g. 1.2.3)")
	flag.BoolVar(&d, "d", false, "enable debug logging")
	flag.Parse()

	if a == "" {
		fmt.Fprintln(os.Stderr, "error: -a (application name) is required")
		flag.Usage()
		os.Exit(1)
	}
	if v == "" {
		fmt.Fprintln(os.Stderr, "error: -v (target version) is required")
		flag.Usage()
		os.Exit(1)
	}

	version, err := patch.NewVersion(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid version %q: %v\n", v, err)
		os.Exit(1)
	}

	fmt.Printf("Patching %s to version %s\n", a, version.String())

	merger, err := patch.CreateMergeTool(mergerBytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error extracting merge tool: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(merger.Dir)

	updates := make(chan gui.PatchUpdate, 32)

	orchestrator := orchestrate.NewOrchestrator(a, version, merger, updates, []orchestrate.OrchestratorStep{
		orchestrate.NewVersionCheckStep(),
		orchestrate.NewManifestDownloadStep(),
		orchestrate.NewUpdatePlanningStep(),
	})

	go orchestrator.Start(context.Background())

	window := gui.NewWindow(a, version, updates)
	go window.Build()

	app.Main()
}
