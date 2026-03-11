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
	_ "modernc.org/sqlite"
	"os"
)

//go:embed bin/hpatchz.exe
var mergerBytes []byte

func main() {

	var a string
	var v string
	var d bool
	var f bool

	flag.StringVar(&a, "a", "", "target application name")
	flag.StringVar(&v, "v", "", "current patch target version")
	flag.BoolVar(&d, "d", false, "enable debugging")
	flag.BoolVar(&f, "f", false, "force patch")
	flag.Parse()

	version, err := patch.NewVersion(v)
	if err != nil {
		panic(err)
	}

	fmt.Println(fmt.Sprintf("Starting patch for application %s to version %s", a, version.String()))
	fmt.Println(fmt.Sprintf("Debugging: %v", d))
	fmt.Println(fmt.Sprintf("Force patch: %v", f))

	var updates = make(chan gui.PatchUpdate, 10)

	fmt.Println("Building merge tool")
	merger, err := patch.CreateMergeTool(mergerBytes)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	defer os.Remove(merger.Path)

	var orchestrator = orchestrate.NewOrchestrator(a, version, updates, []orchestrate.OrchestratorStep{
		orchestrate.NewVersionCheckStep(),
		orchestrate.NewDirectoryScanStep(),
		orchestrate.NewManifestDownloadStep(),
	})

	fmt.Println("Starting patch orchestrator")
	var ctx = context.Background()
	go orchestrator.Start(ctx)

	var window = gui.NewWindow(a, version, updates)
	go window.Build()

	app.Main()

}
