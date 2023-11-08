package app

import "time"

type Result struct {
	wordlenum   int
	userId      string
	displayName string
	score       int
	hardmode    int
	timestamp   time.Time
}
