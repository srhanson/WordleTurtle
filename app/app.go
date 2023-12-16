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
	}
)

// NewHandler creates slack events api handler
// It creates HTTPHandler for development environment
// and LambdaHandler for production env
func NewHandler(c *config.BotConfig) Handler {
	var h Handler
	h = &HTTPHandler{}
	h.Init(c)
	return h

}

// Init initializes handler
func (h *HTTPHandler) Init(c *config.BotConfig) {
	h.config = c
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
			err := h.handleUserMessage(ev)
			if err != nil {
				log.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
	}
}

func (h *HTTPHandler) handleUserMessage(me *slackevents.MessageEvent) error {

	log.Println(me)

	api := slack.New(h.config.SlackBotToken)

	user, err := nameForUser(api, me.User)
	if err != nil {
		return err
	}

	log.Printf("Name=%s\n", user)

	if user == "WordleTurtle" {
		// Our own message, ignore
		return nil
	}

	// TODO - handle errors,
	// Does the text contain a wordle style message?
	res := extractWordleResult(me.Text)
	if res != nil {
		res.userId = me.User
		res.displayName = user

		db, _ := NewDB("./wordles")
		// record it in the database
		db.putResult(*res)
		// Look up the other results for the day
		dailies, _ := db.getDailyResults(res.wordlenum)
		log.Printf("we have %d results", len(dailies))

		// Look up the number of users in the chat (minus wordleturtle)
		// TODO - handle pagination
		params := slack.GetUsersInConversationParameters{ChannelID: me.Channel, Limit: 100}
		users, _, err := api.GetUsersInConversation(&params)
		if err != nil {
			return err
		}
		log.Printf("we have %d users", len(users))
		missing := getMissingPlayers(api, users, dailies)

		if len(dailies) == 1 {
			// Schedule 2 messages: a reminder before deadline and then final results
			go func(exemplar Result) {
				// deadline 5PM PT
				loc, _ := time.LoadLocation("America/Los_Angeles")
				now := time.Now().In(loc)
				deadline := time.Date(now.Year(), now.Month(), now.Day(), 17, 0, 0, 0, loc)
				if now.After(deadline) {
					deadline = deadline.AddDate(0, 0, 1)
				}
				predeadline := deadline.Add(-1 * time.Hour)
				time.Sleep(time.Until(predeadline))
				api := slack.New(h.config.SlackBotToken)
				_, _, err = api.PostMessage(me.Channel, slack.MsgOptionText(":hourglass: 1 hour to deadline! :hourglass:", false))
				time.Sleep(time.Until(deadline))
				// refresh api (not sure if necessary)
				api = slack.New(h.config.SlackBotToken)

				// TODO - this needs a lot of refactor
				db, _ := NewDB("./wordles")
				dailies, _ := db.getDailyResults(res.wordlenum)
				leaders := getLeaders(dailies)
				summaryMsg := makeSummaryPositionMessage(dailies)
				params := slack.GetUsersInConversationParameters{ChannelID: me.Channel, Limit: 100}
				users, _, _ := api.GetUsersInConversation(&params)
				missing := getMissingPlayers(api, users, dailies)

				msg := fmt.Sprintf(":confetti_ball: Congratulations to %s! :confetti_ball:\nFinal %s", leaderString(leaders), summaryMsg)

				if len(missing) > 0 {
					msg += fmt.Sprintf("\n:turkey: %s forgot to show up!", namesString(missing))
				}
				_, _, err = api.PostMessage(me.Channel, slack.MsgOptionText(msg, false))
			}(*res)
		}

		slackPost := getWordlePost(*res, dailies, users, missing)

		_, _, err = api.PostMessage(me.Channel, slack.MsgOptionText(slackPost, false))
		return err
	}
	// TODO - support commands

	return nil
}

// Start starts the server
func (h *HTTPHandler) Start() error {
	if h.config.Env == config.EnvDevelopment {
		return http.ListenAndServe(h.config.BindAddr, nil)
	}
	algnhsa.ListenAndServe(http.DefaultServeMux, nil)
	return nil
}
