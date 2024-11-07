package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"gioui.org/app"
	mevmanifest "github.com/justjack1521/mevmanifest/pkg/genproto"
	"github.com/justjack1521/mevpatch/internal/database"
	"github.com/justjack1521/mevpatch/internal/gui"
	"github.com/justjack1521/mevpatch/internal/manifest"
	"github.com/justjack1521/mevpatch/internal/patch"
	"github.com/justjack1521/mevpatch/internal/update"
	_ "modernc.org/sqlite"
	"os"
	"os/exec"
	"time"
)

//go:embed bin/jojomerge.exe
var jojomergeBytes []byte

func main() {

	var t string
	var v string
	var d bool
	var f bool

	flag.StringVar(&t, "t", "launcher", "target application name")
	flag.StringVar(&v, "v", "1.0.0", "current patch target version")
	flag.BoolVar(&d, "d", false, "enable debugging")
	flag.BoolVar(&f, "f", false, "force patch")
	flag.Parse()

	version, err := patch.NewVersion(v)
	if err != nil {
		panic(err)
	}

	var incrementer = make(chan float32)
	var broadcaster = make(chan string)
	var catcher = make(chan error, 10)

	go func() {
		for {
			time.Sleep(time.Second / 25)
			incrementer <- 0.004
		}
	}()

	go func() {

		var ctx = context.Background()

		var dbe = database.ExistsAtPath()

		if dbe == false {
			broadcaster <- "Downloading database file"
			if err := database.DownloadDatabase("https://mevius-patch-us.sfo3.digitaloceanspaces.com", t, patch.Version{version.Major, version.Minor, 0}); err != nil {
				catcher <- err
				return
			}
		}

		dbc, err := database.NewConnection()
		if err != nil {
			catcher <- err
			return
		}
		dbc.Exec("PRAGMA busy_timeout = 5000;")
		dbc.Exec("PRAGMA journal_mode = WAL;")

		var repository = database.NewPatchingRepository(dbc)
		current, err := repository.GetApplicationVersion(ctx, t)
		if err != nil {
			catcher <- err
			return
		}

		broadcaster <- fmt.Sprintf("Current version found as %s", current.String())

		time.Sleep(time.Second * 2)

		broadcaster <- "Downloading patch manifest..."

		mani, err := manifest.DownloadManifest("https://mevius-patch-us.sfo3.digitaloceanspaces.com", t, version)
		if err != nil {
			catcher <- err
			return
		}

		broadcaster <- fmt.Sprintf("Manifest found for version %s", mani.Version)

		time.Sleep(time.Second * 2)

		fmt.Println(fmt.Sprintf("Starting planning of %d files", len(mani.Files)))

		var planner = update.NewPlanner(t, repository)
		planner.Start(mani.Files)

		return

		var remotes = make([]*update.RemoteFileValidateJob, len(mani.Files))
		for i, file := range mani.Files {
			remotes[i] = update.NewRemoteFileValidateJob(file)
		}

		var remoter = update.NewRemoteFileValidator(t, repository, 1)
		remoter.Start(remotes)

		var bundle *mevmanifest.Bundle

		for _, b := range mani.Bundles {
			if b.Version == current.String() {
				bundle = b
				break
			}
		}

		if bundle == nil {
			launcher, err := update.PersistentPath("Blank Project Launcher.exe")
			if err != nil {
				fmt.Println(err)
				return
			}
			exec.Command(launcher)
			return
		}

		merger, err := patch.CreateMerger(jojomergeBytes)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		defer os.Remove(merger.Path)

		var patcher = update.NewRemoteFilePatcher(t, bundle.Version, bundle.DownloadPath, repository, mani.Files)
		if err := patcher.Start(merger); err != nil {
			fmt.Println(err.Error())
			return
		}

	}()

	var window = gui.NewWindow(t, version, incrementer, broadcaster, catcher)
	go window.Build()

	app.Main()

}
