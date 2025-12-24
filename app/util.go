package app

import (
	"fmt"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

func isCommandMessage(message string) (bool, string) {
	matcher := regexp.MustCompile(`^WordleTurtle (help|leaderboard)`)
	matches := matcher.FindSubmatch([]byte(message))
	if len(matches) == 0 {
		return false, ""
	}
	return true, string(matches[1])
}

func extractWordleResult(message string) *Result {
	matcher := regexp.MustCompile(`^\s*Wordle ([\d,]+).* (\d|x|X)/\d(\*)?`)
	matches := matcher.FindSubmatch([]byte(message))
	if len(matches) == 0 {
		return nil
	}
	var r Result
	wordleNumStr := string(matches[1])
	// Remove commas for thousands separator
	wordleNumStr = strings.Replace(wordleNumStr, ",", "", -1)
	r.wordlenum, _ = strconv.Atoi(wordleNumStr)
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

	summary := fmt.Sprintf("Current %s", summaryMsg)
	// this got too long, so removing it to clean up the content
	//"Waiting on: %s :hourglass:", summaryMsg, strings.Join(missing, ", "))

	res := getSpecialDayAffirmation(current.score)
	if res != "" {
		// It's a special day (like christmas)
		summary = fmt.Sprintf("%s\n\n%s", res, summary)
	} else if len(dailies) == 1 {
		// First person to play, give them a little earlybird message
		summary = fmt.Sprintf("%s\n\n%s", getEarlyBirdMessage(current.score), summary)
	} else if userInLead(current, dailies) {
		summary = fmt.Sprintf("%s\n\n%s", getAffirmation(current.score), summary)
	} else if userInLast(current, dailies) {
		summary = fmt.Sprintf("%s\n\n%s", getConsolation(), summary)
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

func getMissingPlayers(slack SlackConnection, userIds []string, results []Result) []string {
	log.Printf("Users = %s", strings.Join(userIds, ", "))
	missing := make([]string, 0)
OUTER:
	for _, u := range userIds {
		//log.Printf("Checking %s", u)
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
		user, err := slack.NameForUser(u)
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

func getLeaderBoardPost(db DB, slack SlackConnection, wordlenum int, channel string) (string, error) {
	// Get latest wordlenum
	// Get results for previous 7 wordles
	// tabulate scores by user
	// format post text
	users, err := slack.GetUsers(channel)
	if err != nil {
		return "", err
	}

	type LeaderboardScore struct {
		userId      string
		totalScore  int
		scoreMatrix []int
	}

	userScores := make(map[string]*LeaderboardScore)

	LOOKBACK_DAYS := 7
	// Pre-seed userScores
	for _, user := range users {
		userScores[user] = &LeaderboardScore{
			userId:      user,
			totalScore:  0,
			scoreMatrix: make([]int, 8),
		}
		// Start with all turkeys
		userScores[user].scoreMatrix[len(userScores[user].scoreMatrix)-1] = LOOKBACK_DAYS
	}

	for i := 0; i < LOOKBACK_DAYS; i++ {
		dailies, err := db.getDailyResults(wordlenum - i)
		if err != nil {
			return "", err
		}
		for _, result := range dailies {
			us, ok := userScores[result.userId]
			if !ok {
				continue
			}
			us.totalScore += 8 - result.score
			us.scoreMatrix[result.score-1] += 1
			us.scoreMatrix[len(us.scoreMatrix)-1] -= 1
		}
	}

	// Get the sorted scores
	scores := make([]*LeaderboardScore, 0, len(userScores))
	for _, score := range userScores {
		scores = append(scores, score)
	}
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].totalScore > scores[j].totalScore
	})

	tw := table.NewWriter()
	rowHeader := table.Row{"Player", "Score", "1s", "2s", "3s", "4s", "5s", "6s", "Xs", "Turkey"}
	tw.AppendHeader(rowHeader)

	missing := []string{}

	for _, score := range scores {
		player, err := slack.NameForUser(score.userId)
		if err != nil {
			return "", err
		}

		if player == "WordleTurtle" {
			continue
		}
		if score.totalScore == 0 {
			missing = append(missing, player)
			continue
		}

		m := score.scoreMatrix
		tw.AppendRow(table.Row{player, score.totalScore, m[0], m[1], m[2], m[3], m[4], m[5], m[6], m[7]})
	}

	tw.Style().Format = table.FormatOptions{
		Header: text.FormatDefault,
	}

	msg := "```\n" + tw.Render() + "\n```"
	msg += fmt.Sprintf("\n:turkey: %s forgot to show up!", namesString(missing))

	return msg, nil
}

func DefaultLocation() *time.Location {
	loc, _ := time.LoadLocation("America/Los_Angeles")
	return loc
}

func NowDefault() time.Time {
	return time.Now().In(DefaultLocation())
}

func DayForWordle(wordleNum int) time.Time {
	// This might get out of sync at some point if a day ever gets skipped
	sentinel := time.Date(2024, time.December, 23, 0, 0, 0, 0, DefaultLocation())
	sentinelWordle := 1283
	return sentinel.AddDate(0, 0, wordleNum-sentinelWordle)
}

func WordleForDay(now time.Time) int {
	// This might get out of sync at some point if a day ever gets skipped
	sentinel := time.Date(2024, time.December, 23, 0, 0, 0, 0, DefaultLocation())
	sentinelWordle := 1283
	// Daylight savings can probably mess this up...
	elapsedDays := now.Sub(sentinel).Hours() / 24
	return sentinelWordle + int(elapsedDays)
}
