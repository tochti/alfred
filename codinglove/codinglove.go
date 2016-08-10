package codinglove

import (
	"aap/alfred"
	"fmt"
	"os"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/boltdb/bolt"
	"github.com/kelseyhightower/envconfig"
	"github.com/nlopes/slack"
	"github.com/tochti/chief"
	"github.com/uber-go/zap"
)

const (
	Name        = "codinglove"
	PostsBucket = "posts"
)

type (
	codingLove struct {
		stopC     chan struct{}
		jobC      chan chief.Job
		respC     chan alfred.MessageResponse
		ticker    *time.Ticker
		db        *bolt.DB
		url       string
		channelID string
		log       zap.Logger
	}

	specs struct {
		PostsDB  string        `requried:"true" envconfig:"POSTS_DB"`
		Channel  string        `required:"true" envconfig:"CHANNEL"`
		Duration time.Duration `required:"true" envconfig:"DURATION"`
	}
)

func readSpecs() specs {
	s := specs{}
	err := envconfig.Process(Name, &s)
	if err != nil {
		fmt.Printf("%v: Cannot read config - %v", Name, err)
		os.Exit(0)
	}

	return s
}

func New(log zap.Logger) *codingLove {
	log = log.With(zap.String("module", Name))

	s := readSpecs()

	db, err := bolt.Open(s.PostsDB, 0660, nil)
	if err != nil {
		fmt.Printf("Cannot open bolt db - %v", zap.Error(err))
		os.Exit(0)
	}

	return &codingLove{
		ticker:    time.NewTicker(s.Duration),
		db:        db,
		url:       "http://thecodinglove.com/",
		channelID: s.Channel,
		log:       log,
	}
}

func (s *codingLove) Start(jC chan chief.Job) {
	s.stopC = make(chan struct{})
	s.jobC = jC

	s.log.Debug("Start codinglove watcher")
	s.watch()
}

func (s *codingLove) watch() {
	for {
		select {
		case <-s.ticker.C:
			s.readNewPosts()
		case <-s.stopC:
			return
		}
	}
}

func (s *codingLove) readNewPosts() {
	s.log.Debug("Check for new posts")
	doc, err := goquery.NewDocument(s.url)
	if err != nil {
		s.log.Error(
			"Error cannot read codinglove website",
			zap.String("url", s.url),
			zap.Error(err),
		)
		return
	}

	imgs := doc.Find(".post .bodytype .e img")
	titles := doc.Find(".post h3 a")

	if len(titles.Nodes) != len(imgs.Nodes) {
		s.log.Error("Found images are unequal to found titles")
		return
	}

	for i := range imgs.Nodes {
		s.log.Debug("Scan found images")
		sel := imgs.Eq(i)
		key, ok := sel.Attr("src")
		if !ok {
			s.log.Debug("Cannot find attribute 'src' in node")
			continue
		}

		if s.isInDB(key) {
			s.log.Debug("Gif is already in db", zap.String("gif", key))
			continue
		}

		title := titles.Eq(i).Text()

		params := slack.PostMessageParameters{
			AsUser: true,
		}
		params.Attachments = []slack.Attachment{
			slack.Attachment{
				Title:    title,
				ImageURL: key,
			},
		}
		msg := alfred.Message{
			Response:  make(chan alfred.MessageResponse),
			ChannelID: s.channelID,
			Text:      "New codinglove post",
			Params:    params,
		}

		s.jobC <- chief.Job{msg}

		resp := <-msg.Response
		if resp.Err != nil {
			s.log.Error(
				"Error while posting message",
				zap.String("channel_id", resp.ChannelID),
				zap.Error(resp.Err),
			)
			return
		}

		// everything was successfull save gif in db
		s.putPost(key, title)

		s.log.Debug(
			"Send message successfully",
			zap.String("title", title),
			zap.String("gif", key),
			zap.String("channel_id", resp.ChannelID),
		)
	}

}

func (s *codingLove) isInDB(key string) bool {
	exist := false

	s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(PostsBucket))
		if b == nil {
			return nil
		}
		p := b.Get([]byte(key))
		if len(p) != 0 {
			exist = true
		}

		return nil
	})
	return exist
}

func (s *codingLove) putPost(key, title string) {
	s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(PostsBucket))
		if err != nil {
			s.log.Error("Cannot find bucket", zap.Error(err))
			return err
		}
		err = b.Put([]byte(key), []byte(title))
		if err != nil {
			s.log.Error("Cannot put post to db", zap.Error(err))
		}

		return err
	})
}

func (s *codingLove) Stop() {
	s.stopC <- struct{}{}
	s.ticker.Stop()
	s.log.Debug("Stopped!")
}
