package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractWordle(t *testing.T) {
	res := extractWordleResult("Wordle 123 4/6")
	assert.Equal(t, 123, res.wordlenum)
	assert.Equal(t, 4, res.score)
	assert.Equal(t, 0, res.hardmode)

	res = extractWordleResult("Wordle 123 x/6*")
	assert.Equal(t, 123, res.wordlenum)
	assert.Equal(t, 7, res.score)
	assert.Equal(t, 1, res.hardmode)

	res = extractWordleResult("Wordle 867 X/6")
	assert.Equal(t, 867, res.wordlenum)
	assert.Equal(t, 7, res.score)
	assert.Equal(t, 0, res.hardmode)
}

func TestExtractWordle_NoMatch(t *testing.T) {
	res := extractWordleResult("test")
	assert.Nil(t, res)

	res = extractWordleResult("Reshare of\nWordle 867 X/6")
	assert.Nil(t, res)
}

func Test_makeSummaryPositionMessage(t *testing.T) {
	inputs := []Result{
		{score: 5, displayName: "user1", wordlenum: 123},
		{score: 3, displayName: "user2", wordlenum: 123},
		{score: 3, displayName: "user3", wordlenum: 123},
		{score: 7, displayName: "user4", wordlenum: 123},
	}
	res := makeSummaryPositionMessage(inputs)
	expected := "Results for Wordle #123:\n3/6: user2, user3\n5/6: user1\nx/6: user4\n"
	assert.Equal(t, expected, res)
}
