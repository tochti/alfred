package main

import (
	"aap/alfred"
	"aap/alfred/codinglove"
	"flag"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/nlopes/slack"
)

func main() {
	token := flag.String("token", "", "Slack Bot API Token")
	flag.Parse()

	if *token == "" {
		fmt.Println("Token is missing!")
		return
	}

	// Init slack client
	api := slack.New(*token)
	logger := log.New(os.Stdout, "slack-bot: ", log.Lshortfile|log.LstdFlags)
	slack.SetLogger(logger)
	api.SetDebug(true)

	// Init codinglove sender
	cl := codinglove.New(logger, 5*time.Second, "./posts.bolt", "@ttochtermann")

	// Init alfred
	b := &alfred.Butler{
		API: api,
		WG:  sync.WaitGroup{},
	}
	b.NewSender(cl)

	// Init SIGHUP event
	alfred.WatchSIGHUP(b)

	b.Serve()
}
