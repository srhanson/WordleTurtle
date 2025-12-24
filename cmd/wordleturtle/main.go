package main

import (
	"log"
	"wordleturtle/app"
	"wordleturtle/config"
)

func main() {
	c, err := config.Parse()
	if err != nil {
		log.Fatal(err)
	}

	handler := app.NewHandler(c)
	log.Fatal(handler.Start())
}
