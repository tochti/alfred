package codinglove

import (
	"aap/alfred"
	"log"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/boltdb/bolt"
	"github.com/nlopes/slack"
	"github.com/tochti/chief"
)

const (
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
		log       *log.Logger
	}
)

func New(log *log.Logger, d time.Duration, index string, channelID string) *codingLove {
	log.SetPrefix("codinglove: ")

	db, err := bolt.Open(index, 0660, nil)
	if err != nil {
		log.Fatalf("Cannot open bolt db - %v", err)
	}

	return &codingLove{
		ticker:    time.NewTicker(d),
		db:        db,
		url:       "http://thecodinglove.com/",
		channelID: channelID,
		log:       log,
	}
}

func (s *codingLove) Start(jC chan chief.Job) {
	s.stopC = make(chan struct{})
	s.jobC = jC

	s.watch()
}

func (s *codingLove) watch() {
	for {
		select {
		case <-s.ticker.C:
			go s.readNewPosts()
		case <-s.stopC:
			return
		}
	}
}

func (s *codingLove) readNewPosts() {
	s.log.Println("Check for new posts")
	doc, err := goquery.NewDocument(s.url)
	if err != nil {
		s.log.Printf("Error cannot read %s: %v\n", s.url, err)
		return
	}

	imgs := doc.Find(".post .bodytype .e img")
	titles := doc.Find(".post h3 a")

	if len(titles.Nodes) != len(imgs.Nodes) {
		s.log.Printf("Found images are unequal to found titles")
		return
	}

	for i := range imgs.Nodes {
		s.log.Printf("Scan images")
		sel := imgs.Eq(i)
		key, ok := sel.Attr("src")
		if !ok {
			s.log.Printf("Cannot find src attr in node %v", sel)
			continue
		}

		if s.isInDB(key) {
			s.log.Printf("%v is already in db", key)
			continue
		}

		title := titles.Eq(i).Text()

		params := slack.PostMessageParameters{}
		params.Attachments = []slack.Attachment{
			slack.Attachment{
				Title:    title,
				ImageURL: key,
			},
		}
		msg := alfred.Message{
			Response:  make(chan alfred.MessageResponse),
			ChannelID: s.channelID,
			Params:    params,
		}

		s.jobC <- chief.Job{msg}

		resp := <-msg.Response
		if resp.Err != nil {
			s.log.Printf("Error while posting message on channel %v - %v", resp.ChannelID, resp.Err)
			return
		}

		// everything was successfull save gif in db
		s.putPost(key, title)

		s.log.Printf("Send %v %v to channel %v", title, key, resp.ChannelID)
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
			return err
		}
		err = b.Put([]byte(key), []byte(title))
		return err
	})
}

func (s *codingLove) Stop() {
	s.stopC <- struct{}{}
	s.ticker.Stop()
	s.log.Println("Stopped!")
}
