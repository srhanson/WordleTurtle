package app

import (
	"fmt"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/slack-go/slack"
)

func extractWordleResult(message string) *Result {
	matcher := regexp.MustCompile(`^Wordle (\d+) (\d|x|X)/\d(\*)?`)
	matches := matcher.FindSubmatch([]byte(message))
	if len(matches) == 0 {
		return nil
	}
	var r Result
	r.wordlenum, _ = strconv.Atoi(string(matches[1]))
	scoreStr := string(matches[2])
	if scoreStr == "x" || scoreStr == "X" {
		r.score = 7
	} else {
		r.score, _ = strconv.Atoi(scoreStr)
	}
	r.hardmode = len(matches[3])
	return &r
}

func getLeaders(results []Result) []Result {
	bestscore := 8
	for _, r := range results {
		if r.score < bestscore {
			bestscore = r.score
		}
	}

	var leaders []Result
	for _, r := range results {
		if r.score == bestscore {
			leaders = append(leaders, r)
		}
	}

	return leaders
}

func namesString(names []string) string {
	if len(names) == 1 {
		return names[0]
	}
	suffix := names[len(names)-2] + " and " + names[len(names)-1]
	prefix := ""
	for _, l := range names[:len(names)-2] {
		prefix += l + ", "
	}
	return prefix + suffix
}

func leaderString(leaders []Result) string {
	names := make([]string, 0)
	for _, l := range leaders {
		names = append(names, l.displayName)
	}

	return namesString(names)
}

func getWordlePost(current Result, dailies []Result, users, missing []string) string {
	leaders := getLeaders(dailies)
	leaderStr := leaderString(leaders)

	// All users played (except the bot)?
	summaryMsg := makeSummaryPositionMessage(dailies)

	if len(missing) == 0 {
		return fmt.Sprintf(":confetti_ball: Congratulations to %s! :confetti_ball:\nFinal %s", leaderStr, summaryMsg)
	}

	summary := fmt.Sprintf("Current %s\nWaiting on: %s :hourglass:", summaryMsg, strings.Join(missing, ", "))

	if len(dailies) == 1 {
		// First person to play, give them a little earlybird message
		summary = fmt.Sprintf("%s\n%s", getEarlyBirdMessage(current.score), summary)
	} else if userInLead(current, dailies) {
		summary = fmt.Sprintf("%s\n%s", getAffirmation(current.score), summary)
	} else if userInLast(current, dailies) {
		summary = fmt.Sprintf("%s\n%s", getConsolation(), summary)
	}
	return summary
}

func userInLead(result Result, results []Result) bool {
	if len(results) == 0 {
		return false
	}
	sort.Slice(results, func(i, j int) bool { return results[i].score < results[j].score })

	return result.score == results[0].score
}

func userInLast(result Result, results []Result) bool {
	if len(results) == 0 {
		return false
	}
	sort.Slice(results, func(i, j int) bool { return results[i].score < results[j].score })

	return result.score == results[len(results)-1].score
}

func makeSummaryPositionMessage(results []Result) string {
	if len(results) == 0 {
		return "No plays yet."
	}
	sort.Slice(results, func(i, j int) bool { return results[i].score < results[j].score })

	message := fmt.Sprintf("Results for Wordle #%d:\n", results[0].wordlenum)

	appendPosition := func(pos int, users []string) {
		if len(users) == 0 {
			return
		}
		posStr := fmt.Sprint(pos)
		if pos > 6 {
			posStr = "x"
		}
		message += fmt.Sprintf("%s/6: %s\n", posStr, strings.Join(users, ", "))
	}

	currentPos := 0
	currentUsers := make([]string, 0)
	for _, r := range results {
		if r.score > currentPos && len(currentUsers) > 0 {
			appendPosition(currentPos, currentUsers)
			currentUsers = make([]string, 0)
		}
		currentPos = r.score
		currentUsers = append(currentUsers, r.displayName)
	}
	appendPosition(currentPos, currentUsers)

	return message
}

func nameForUser(api *slack.Client, userId string) (string, error) {
	user, err := api.GetUserInfo(userId)
	if err != nil {
		return "", err
	}
	// too many names
	names := []string{user.Profile.DisplayName, user.Profile.DisplayNameNormalized, user.Profile.FirstName, user.Profile.RealName, user.Profile.RealNameNormalized}
	for _, n := range names {
		if n != "" {
			return n, nil
		}
	}
	return userId, nil
}

func getMissingPlayers(api *slack.Client, userIds []string, results []Result) []string {
	log.Printf("Users = %s", strings.Join(userIds, ", "))
	missing := make([]string, 0)
OUTER:
	for _, u := range userIds {
		log.Printf("Checking %s", u)
		for _, r := range results {
			if r.userId == u {
				log.Printf("Found match %v", r)
				continue OUTER
			}
		}
		missing = append(missing, u)
	}

	log.Printf("Missing: %s", strings.Join(missing, ", "))

	translated := make([]string, 0, len(missing))
	for _, u := range missing {
		user, err := nameForUser(api, u)
		if err != nil {
			continue
		}
		if user == "WordleTurtle" {
			continue
		}
		translated = append(translated, user)
	}
	log.Printf("Translated: %s", strings.Join(translated, ", "))

	return translated
}
