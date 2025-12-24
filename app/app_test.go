package app

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// =====
// Mocks
// =====

type MockSlack struct {
	mock.Mock
}

func (m *MockSlack) NameForUser(userId string) (string, error) {
	args := m.Called(userId)
	return args.String(0), args.Error(1)
}

func (m *MockSlack) PostMessage(channel, msg string) error {
	args := m.Called(channel, msg)
	return args.Error(0)
}

func (m *MockSlack) GetUsers(channel string) ([]string, error) {
	args := m.Called(channel)
	return args.Get(0).([]string), args.Error(1)
}

type MockDB struct {
	mock.Mock
}

func (m *MockDB) putResult(result Result) error {
	args := m.Called(result)
	return args.Error(0)
}

func (m *MockDB) getDailyResults(wordlenum int) ([]Result, error) {
	args := m.Called(wordlenum)
	return args.Get(0).([]Result), args.Error(1)
}

func (m *MockDB) getLargestWordle() (int, error) {
	args := m.Called()
	return args.Int(0), args.Error(1)
}

// =======
// Helpers
// =======

func makeResult(id, name string, wordlenum, score int) Result {
	return Result{
		wordlenum:   wordlenum,
		userId:      id,
		displayName: name,
		score:       score,
		hardmode:    0,
	}
}

// =====
// Tests
// =====

func Test_handlesWordle(t *testing.T) {
	mockDb := new(MockDB)
	mockSlack := new(MockSlack)

	h := &HTTPHandler{
		config: nil,
		db:     mockDb,
		slack:  mockSlack,
	}

	sm := SlackMessage{
		channel: "testchannel",
		text:    "Wordle 917 3/6*",
		user:    "userid1",
	}

	mockSlack.On("GetUsers", "testchannel").Return([]string{"userid1", "userid2"}, nil)
	mockSlack.On("NameForUser", "userid1").Return("sean", nil)
	mockSlack.On("NameForUser", "userid2").Return("lara", nil)

	resultMatcher := mock.MatchedBy(func(msg string) bool {
		matches, err := regexp.Match(`^(?s).*\n3/6: sean.*`, []byte(msg))
		return matches && err == nil
	})
	mockSlack.On("PostMessage", "testchannel", resultMatcher).Return(nil)

	expectedResult := Result{
		wordlenum:   917,
		userId:      "userid1",
		displayName: "sean",
		score:       3,
		hardmode:    1,
	}
	mockDb.On("putResult", expectedResult).Return(nil)
	mockDb.On("getDailyResults", 917).Return([]Result{expectedResult}, nil)

	assert.Nil(t, h.handleUserMessage(sm))

	// Verify the deadline has been scheduled
	_, ok := scheduled_wordles[917]
	assert.True(t, ok)
}

func Test_handlesCommand(t *testing.T) {
	mockDb := new(MockDB)
	mockSlack := new(MockSlack)

	h := &HTTPHandler{
		config: nil,
		db:     mockDb,
		slack:  mockSlack,
	}

	sm := SlackMessage{
		channel: "testchannel",
		text:    "WordleTurtle help",
		user:    "userid1",
	}

	resultMatcher := mock.MatchedBy(func(msg string) bool {
		matches, err := regexp.Match(`^Supported commands are:`, []byte(msg))
		return matches && err == nil
	})
	mockSlack.On("PostMessage", "testchannel", resultMatcher).Return(nil)
	mockSlack.On("NameForUser", "userid1").Return("sean", nil)

	assert.Nil(t, h.handleUserMessage(sm))
}

func Test_handlesCommand_Leaderboard(t *testing.T) {
	mockDb := new(MockDB)
	mockSlack := new(MockSlack)

	h := &HTTPHandler{
		config: nil,
		db:     mockDb,
		slack:  mockSlack,
	}

	sm := SlackMessage{
		channel: "testchannel",
		text:    "WordleTurtle leaderboard",
		user:    "userid1",
	}

	mockSlack.On("GetUsers", "testchannel").Return([]string{"userid1", "userid2", "userid3"}, nil)
	mockSlack.On("NameForUser", "userid1").Return("sean", nil)
	mockSlack.On("NameForUser", "userid2").Return("lara", nil)
	mockSlack.On("NameForUser", "userid3").Return("grandma", nil)

	resultMatcher := mock.MatchedBy(func(msg string) bool {
		matches, err := regexp.Match(`(?s).*Player.*sean.*25.*grandma.*22.*lara.*20`, []byte(msg))
		return matches && err == nil
	})
	mockSlack.On("PostMessage", "testchannel", resultMatcher).Return(nil)

	mockDb.On("getLargestWordle").Return(917, nil)

	results := [][]Result{
		{
			makeResult("userid1", "sean", 917, 3),
			makeResult("userid2", "lara", 917, 3),
			makeResult("userid3", "grandma", 917, 3),
		},
		{
			makeResult("userid1", "sean", 916, 1),
			makeResult("userid2", "lara", 916, 3),
			makeResult("userid3", "grandma", 916, 5),
		},
		{
			makeResult("userid1", "sean", 915, 7),
			makeResult("userid3", "grandma", 915, 3),
		},
		{
			makeResult("userid1", "sean", 914, 2),
			makeResult("userid2", "lara", 914, 5),
			makeResult("userid3", "grandma", 914, 6),
		},
		{
			makeResult("userid1", "sean", 913, 4),
			makeResult("userid2", "lara", 913, 3),
			makeResult("userid3", "grandma", 913, 2),
		},
		{
			makeResult("userid1", "sean", 912, 6),
			makeResult("userid2", "lara", 912, 6),
			makeResult("userid3", "grandma", 912, 7),
		},
		{},
	}

	mockDb.On("getDailyResults", 917).Return(results[0], nil)
	mockDb.On("getDailyResults", 916).Return(results[1], nil)
	mockDb.On("getDailyResults", 915).Return(results[2], nil)
	mockDb.On("getDailyResults", 914).Return(results[3], nil)
	mockDb.On("getDailyResults", 913).Return(results[4], nil)
	mockDb.On("getDailyResults", 912).Return(results[5], nil)
	mockDb.On("getDailyResults", 911).Return(results[6], nil)

	assert.Nil(t, h.handleUserMessage(sm))
}
