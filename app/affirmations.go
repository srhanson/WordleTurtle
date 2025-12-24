package app

import (
	"math/rand"
	"time"
)

func getRandomString(choices []string) string {
	return choices[rand.Intn(len(choices))]
}

func getEarlyBirdMessage(score int) string {
	early_bird_good := []string{
		"Starting the day strong! :muscle:",
		"Wow! Give everyone else a chance!",
	}
	early_bird_ok := []string{
		"First to play gets first place! :first_place_medal:",
		"Early bird gets the lead! :hatching_chick:",
	}
	early_bird_bad := []string{
		"In the lead (for now!)",
		":thinking_face: Not sure that one will hold...",
		"Good luck staying the in lead with that! :crossed_fingers:",
	}
	if score < 3 {
		return getRandomString(early_bird_good)
	} else if score < 5 {
		return getRandomString(early_bird_ok)
	}
	return getRandomString(early_bird_bad)
}

func getAffirmation(score int) string {
	affirmations_good := []string{
		":star2: Superstar! :star2:",
		"Bish, bash, bosh! :brain:",
		"What a play! :star-struck:",
		"Cowabunga Dude! :tmnt-celebrate:",
		"Jolly good show! :british:",
		"That's gonna be tough to beat! :dart:",
		"By the bushy beard of Thor! :thor:",
	}
	affirmations_bad := []string{
		"In the lead (for now!)",
		":thinking_face: Not sure that one will hold...",
		"Good luck staying the in lead with that! :crossed_fingers:",
	}
	if score < 4 {
		return getRandomString(affirmations_good)
	}
	return getRandomString(affirmations_bad)
}

func getConsolation() string {
	consolations := []string{
		"Can't win 'em all! :cold_sweat:",
		"That one seemed hard for you :melting_face:",
		"You'll get 'em next time (maybe) :shrug-old:",
		"Plays like that are why participation trophies were created :clowntrophy:",
	}
	return getRandomString(consolations)
}

func getSpecialDayAffirmation(score int) string {
	today := NowDefault()
	if today.Day() == 25 && today.Month() == time.December {
		var messages []string
		if score <= 4 {
			messages = append(messages, "It's a Christmas Miracle!! :christmas_tree:")
			messages = append(messages, "Ho ho ho! You are on the nice list! :santa:")
			messages = append(messages, "Grinch isn't the only one to steal Christmas :grinch:")
			messages = append(messages, "Yippee-ki-yay Mother Hubbard! :diehardxmas:")
		} else {
			messages = append(messages, "Jingle Bells, that score smells, try another day! :bell:")
			messages = append(messages, "A stocking full of coal for plays like that :coal:")
		}
		return getRandomString(messages)
	}
	return ""
}
