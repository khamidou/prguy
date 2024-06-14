package main

import (
	"fmt"

	"github.com/getlantern/systray"
    "github.com/ncruces/zenity"
)

func main() {
	onExit := func() {
	}

	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetTemplateIcon([]byte("🕴️"), []byte("🕴️"))
	systray.SetTitle("PR Guy")
	//systray.SetTooltip("")
	mDoSetup := systray.AddMenuItem("GitHub setup", "Authenticate yourself to be able to see your pull requests.")
	mQuitOrig := systray.AddMenuItem("Quit", "Quit the app")

	go func() {
        for {
            select {
            case <-mQuitOrig.ClickedCh:
                fmt.Println("Requesting quit")
                systray.Quit()
                fmt.Println("Finished quitting")
            case <-mDoSetup.ClickedCh:
                fmt.Println("Performing setup")
                startGithubDeviceAuth()
            }
        }
	}()
}

func startGithubDeviceAuth() {
    res, err := zenity.Entry("Enter new text:", zenity.Title("Add a new entry"))
    if err != zenity.ErrCanceled {
        fmt.Println("got an err", err)
    }

    fmt.Println("res", res)
}
