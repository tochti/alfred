package main

import (
	"aap/alfred"
	"aap/alfred/codinglove"
	"log"
	"os"
	"sync"

	"github.com/nlopes/slack"
	"github.com/uber-go/zap"
)

func main() {
	alfredSpecs := alfred.ReadSpecs()

	// Init loggers
	logger := log.New(os.Stdout, "slack-bot: ", log.Lshortfile|log.LstdFlags) // slack logger
	zapLog := zap.New(zap.NewJSONEncoder())

	// Init slack client
	api := slack.New(alfredSpecs.SlackToken)
	slack.SetLogger(logger)

	if alfredSpecs.Debug {
		api.SetDebug(true)
		zapLog.SetLevel(zap.DebugLevel)
	}

	// Init codinglove sender
	cl := codinglove.New(zapLog)

	// Init alfred
	b := &alfred.Butler{
		API: api,
		WG:  sync.WaitGroup{},
		Log: zapLog,
	}
	b.NewSender(cl)

	// Init SIGHUP event
	alfred.WatchKillSignals(b)

	b.Serve()
}
