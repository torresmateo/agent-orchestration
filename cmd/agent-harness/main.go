package main

import (
	"log"
	"os"

	"github.com/mateo/agentvm/internal/harness"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("Agent harness starting...")

	daemon, err := harness.NewDaemon()
	if err != nil {
		log.Printf("Failed to initialize harness: %v", err)
		// Don't exit with error if task config doesn't exist yet—
		// the harness might start before config is injected
		if os.IsNotExist(err) {
			log.Println("No task config found, waiting for dispatch...")
			select {} // block forever, systemd will restart us
		}
		os.Exit(1)
	}

	if err := daemon.Run(); err != nil {
		log.Printf("Harness failed: %v", err)
		os.Exit(1)
	}
}
