package app

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
	"wordleturtle/config"

	"github.com/akrylysov/algnhsa"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

var scheduled_wordles map[int]struct{} = make(map[int]struct{})

type (
	// Handler is an interface for the webserver that handles
	// incoming requests from Slack events API
	//
	// You can add support of any cloud provider by implementing this interface
	Handler interface {
		Init(c *config.BotConfig)
		Start() error
	}
	// HTTPHandler is an implementation of webserver for local development/testing
	HTTPHandler struct {
		Handler
		config *config.BotConfig
		db     DB
		slack  SlackConnection
	}
)

type SlackMessage struct {
	channel string
	user    string
	text    string
}

func ConvertSlackMessage(me slackevents.MessageEvent) SlackMessage {
	return SlackMessage{
		channel: me.Channel,
		user:    me.User,
		text:    me.Text,
	}
}

// NewHandler creates slack events api handler
// It creates HTTPHandler for development environment
func NewHandler(c *config.BotConfig) Handler {
	h := &HTTPHandler{}
	h.Init(c)
	return h

}

// Init initializes handler
func (h *HTTPHandler) Init(c *config.BotConfig) {
	h.config = c
	h.db, _ = NewSQLiteDB("./wordles")
	h.slack = NewSlackAPIConnection(h.config.SlackBotToken)
	http.HandleFunc("/", h.handle)
}

// handle handles incoming data from
func (h *HTTPHandler) handle(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)

	log.Printf("Got request: %v", string(body))
	log.Printf("Got headers: %v", r.Header)

	if err != nil {
		log.Print(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	sv, err := slack.NewSecretsVerifier(r.Header, h.config.SigningSecret)
	if err != nil {
		log.Print(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, err := sv.Write(body); err != nil {
		log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if err := sv.Ensure(); err != nil {
		log.Print(err)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionNoVerifyToken())
	if err != nil {
		log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if eventsAPIEvent.Type == slackevents.URLVerification {
		var r *slackevents.ChallengeResponse
		err := json.Unmarshal([]byte(body), &r)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text")
		w.Write([]byte(r.Challenge))
	}
	if eventsAPIEvent.Type == slackevents.CallbackEvent {
		innerEvent := eventsAPIEvent.InnerEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.MessageEvent:
			log.Println(ev)
			err := h.handleUserMessage(ConvertSlackMessage(*ev))
			if err != nil {
				log.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
	}
}

func (h *HTTPHandler) handleUserMessage(sm SlackMessage) error {
	user, err := h.slack.NameForUser(sm.user)
	if err != nil {
		return err
	}

	log.Printf("Name=%s\n", user)

	if user == "WordleTurtle" {
		// Our own message, ignore
		return nil
	}

	// Commands - Message starts with @WordleTurtle
	iscmd, cmd := isCommandMessage(sm.text)
	if iscmd {
		return h.handleCommand(sm, cmd)
	}

	// TODO - handle errors,
	// Does the text contain a wordle style message?
	res := extractWordleResult(sm.text)
	if res != nil {
		res.userId = sm.user
		res.displayName = user
		return h.handleWordle(sm, res)
	}

	return nil
}

func (h *HTTPHandler) handleCommand(sm SlackMessage, cmd string) error {
	var err error
	switch cmd {
	case "help":
		commands := `Supported commands are:
help - display this help text
leaderboard - display the weekly leaderboard`
		err = h.slack.PostMessage(sm.channel, commands)
	case "leaderboard":
		wordlenum, err := h.db.getLargestWordle()
		if err != nil {
			return err
		}
		slackPost, err := getLeaderBoardPost(h.db, h.slack, wordlenum, sm.channel)
		if err != nil {
			return err
		}
		err = h.slack.PostMessage(sm.channel, slackPost)
		if err != nil {
			return err
		}
	}
	return err
}

func (h *HTTPHandler) handleWordle(sm SlackMessage, res *Result) error {

	// record it in the database
	h.db.putResult(*res)
	// Look up the other results for the day
	dailies, _ := h.db.getDailyResults(res.wordlenum)
	log.Printf("we have %d results", len(dailies))

	// Look up the number of users in the chat (minus wordleturtle)
	// TODO - handle pagination
	users, err := h.slack.GetUsers(sm.channel)
	if err != nil {
		return err
	}
	log.Printf("we have %d users", len(users))
	missing := getMissingPlayers(h.slack, users, dailies)

	// If we haven't scheduled a deadline message, do so now
	if _, ok := scheduled_wordles[res.wordlenum]; !ok {
		// TODO - block old wordles from being posted and getting a deadline
		scheduled_wordles[res.wordlenum] = struct{}{}
		// Schedule 2 messages: a reminder before deadline and then final results
		go h.postEndOfDay(*res, sm.channel)
	}

	slackPost := getWordlePost(*res, dailies, users, missing)
	err = h.slack.PostMessage(sm.channel, slackPost)
	return err
}

func (h *HTTPHandler) postEndOfDay(exemplar Result, channel string) {
	log.Printf("Scheduling deadline for wordle %d", exemplar.wordlenum)
	// deadline 5PM PT
	base := DayForWordle(exemplar.wordlenum)
	now := NowDefault()

	if now.Sub(base).Hours() > 24 {
		log.Printf("Not scheduling old wordle: %v", exemplar)
		return
	}
	deadline := time.Date(base.Year(), base.Month(), base.Day(), 17, 0, 0, 0, base.Location())
	if base.After(deadline) {
		deadline = deadline.AddDate(0, 0, 1)
	}
	predeadline := deadline.Add(-1 * time.Hour)

	log.Printf("Sleeping until predeadline for wordle %d: %s", exemplar.wordlenum, predeadline.String())

	time.Sleep(time.Until(predeadline))
	h.slack.PostMessage(channel, fmt.Sprintf(":hourglass: 1 hour to deadline for Wordle #%d! :hourglass:", exemplar.wordlenum))
	time.Sleep(time.Until(deadline))

	dailies, _ := h.db.getDailyResults(exemplar.wordlenum)
	leaders := getLeaders(dailies)
	summaryMsg := makeSummaryPositionMessage(dailies)

	users, _ := h.slack.GetUsers(channel)
	missing := getMissingPlayers(h.slack, users, dailies)

	msg := fmt.Sprintf(":confetti_ball: Congratulations to %s! :confetti_ball:\nFinal %s", leaderString(leaders), summaryMsg)

	if len(missing) > 0 {
		msg += fmt.Sprintf("\n:turkey: %s forgot to show up!", namesString(missing))
	}
	h.slack.PostMessage(channel, msg)

	// If Saturday, post the weekly leaderboard
	if base.Weekday() == 6 {
		leaderboard, err := getLeaderBoardPost(h.db, h.slack, exemplar.wordlenum, channel)
		if err == nil {
			slackPost := "Weekly Leaderboard\n" + leaderboard
			h.slack.PostMessage(channel, slackPost)
		}
	}
}

// Start starts the server
func (h *HTTPHandler) Start() error {
	if h.config.Env == config.EnvDevelopment {
		return http.ListenAndServe(h.config.BindAddr, nil)
	}
	algnhsa.ListenAndServe(http.DefaultServeMux, nil)
	return nil
}
