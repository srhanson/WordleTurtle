package app

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
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

		leaders := getLeaders(dailies)
		leaderStr := leaderString(leaders)

		// All users played (except the bot)?
		remaining := len(users) - len(dailies) - 1
		summaryMsg := makeSummaryPositionMessage(dailies)
		if remaining <= 0 {
			winners := fmt.Sprintf("Congratulations to %s!\nFinal %s", leaderStr, summaryMsg)
			_, _, err = api.PostMessage(me.Channel, slack.MsgOptionText(winners, false))
			return err
		}

		missing := getMissingPlayers(api, users, dailies)
		summary := fmt.Sprintf("Current %s\nWaiting on: %s", summaryMsg, strings.Join(missing, ", "))
		_, _, err = api.PostMessage(me.Channel, slack.MsgOptionText(summary, false))
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
