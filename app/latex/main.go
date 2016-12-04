package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/nilbot/slackbot"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: latex slack-bot-token\n")
		os.Exit(1)
	}
	token := os.Args[1]
	ws, _ := slack.Connect(token)
	fmt.Println("latex slackbot running, ^C to terminate")

	for {
		m, err := slack.GetMessage(ws)
		if err != nil {
			log.Fatal(err)
		}
		if m.Type == "message" && isValidLatex(m.Text) {

		}
	}
}

func isValidLatex(candidate string) bool {
	if !strings.HasPrefix(candidate, `$`) || !strings.HasPrefix(candidate, `\begin{`) {
		return false
	}
	return false
}
