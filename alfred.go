package alfred

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/nlopes/slack"
	"github.com/tochti/chief"
)

type (
	Sender interface {
		// start sender. sender muss nicht im Hintergrund gestart werden wird vom Butler verwaltet werden.
		Start(chan chief.Job)
		// stop sender
		Stop()
	}

	Message struct {
		Response  chan MessageResponse
		ChannelID string
		Text      string
		Params    slack.PostMessageParameters
	}

	MessageResponse struct {
		ChannelID string
		Timestamp string
		Err       error
	}

	Butler struct {
		senders []Sender
		API     *slack.Client
		WG      sync.WaitGroup
	}
)

func (b *Butler) NewSender(s Sender) {
	b.senders = append(b.senders, s)
}

func (b *Butler) Serve() {

	c := chief.New(5, b.postMessage)
	c.Start()

	for _, s := range b.senders {
		b.WG.Add(1)
		go func() {
			defer b.WG.Done()
			s.Start(c.Jobs)
		}()
	}

	b.WG.Wait()
}

func (b *Butler) postMessage(j chief.Job) {
	msg, ok := j.Order.(Message)
	if !ok {
		return
	}

	c, t, err := b.API.PostMessage(msg.ChannelID, msg.Text, msg.Params)
	go func() {
		msg.Response <- MessageResponse{
			ChannelID: c,
			Timestamp: t,
			Err:       err,
		}
	}()
}

func (b *Butler) Stop() {
	wg := sync.WaitGroup{}
	for _, s := range b.senders {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Stop()
		}()
	}

	wg.Wait()
}

func WatchSIGHUP(b *Butler) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP, os.Interrupt)

	go func() {
		<-sig
		// If it no possible to stop all sender kill the bot
		time.AfterFunc(5*time.Second, func() {
			os.Exit(1)
		})

		b.Stop()
	}()
}
