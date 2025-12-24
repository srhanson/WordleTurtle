package app

import (
	"sync"

	"github.com/slack-go/slack"
)

type SlackConnection interface {
	NameForUser(userId string) (string, error)
	PostMessage(channel, msg string) error
	GetUsers(channel string) ([]string, error)
}

type SlackAPIConnection struct {
	api       *slack.Client
	nameCache sync.Map
}

func NewSlackAPIConnection(slackToken string) *SlackAPIConnection {
	api := slack.New(slackToken)
	if api == nil {
		panic("Failed to connect to slack")
	}
	return &SlackAPIConnection{api: api}

}

func (s *SlackAPIConnection) NameForUser(userId string) (string, error) {
	if res, ok := s.nameCache.Load(userId); ok {
		return res.(string), nil
	}
	user, err := s.api.GetUserInfo(userId)
	if err != nil {
		return "", err
	}
	// too many names
	names := []string{user.Profile.DisplayName, user.Profile.DisplayNameNormalized, user.Profile.FirstName, user.Profile.RealName, user.Profile.RealNameNormalized}
	for _, n := range names {
		if n != "" {
			s.nameCache.Store(userId, n)
			return n, nil
		}
	}
	return userId, nil
}

func (s *SlackAPIConnection) PostMessage(channel, msg string) error {
	_, _, err := s.api.PostMessage(channel, slack.MsgOptionText(msg, false))
	return err
}

func (s *SlackAPIConnection) GetUsers(channel string) ([]string, error) {
	params := slack.GetUsersInConversationParameters{ChannelID: channel, Limit: 100}
	users, _, err := s.api.GetUsersInConversation(&params)
	return users, err
}
