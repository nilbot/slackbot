package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nilbot/gophernews"
	"github.com/nilbot/slackbot"
)

// URI : https://hacker-news.firebaseio.com/v0/topstories.json?print=pretty
func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: hn slack-bot-token\n")
		os.Exit(1)
	}

	// start a websocket-based Real Time API session
	ws, id := slack.Connect(os.Args[1])
	fmt.Println("hn ready, ^C exits")

	for {
		// read each incoming message
		m, err := slack.GetMessage(ws)
		if err != nil {
			log.Fatal(err)
		}

		// see if we're mentioned
		if m.Type == "message" && strings.HasPrefix(m.Text, "<@"+id+">") {
			// if so try to parse if
			parts := strings.Fields(m.Text)
			if len(parts) > 1 && parts[1] == "news" {
				// getting news
				if len(parts) == 2 {
					go func(m slack.Message) {
						m.Text = getTopNews("3")
						slack.PostMessage(ws, m)
					}(m)
				} else {
					go func(m slack.Message) {
						m.Text = getTopNews(parts[2])
						slack.PostMessage(ws, m)
					}(m)
				}
				// NOTE: the Message object is copied, this is intentional
			} else if len(parts) == 3 && parts[1] == "stock" {
				// getting stock
				go func(m slack.Message) {
					m.Text = getQuote(parts[2])
					slack.PostMessage(ws, m)
				}(m)
			} else {
				// huh?
				m.Text = fmt.Sprintf("sorry, can't serve you anything except 'news #{top n}' and 'stock #{ticker}' for now.\n")
				slack.PostMessage(ws, m)
			}
		}
	}
}

var HNItemURLPrefix string = "https://news.ycombinator.com/item?id="

func getTopNews(topN string) string {
	n, err := strconv.Atoi(topN)
	if err != nil {
		return fmt.Sprintf("top n parsed error: %v", err)
	}
	if n > 5 {
		n = 5
	}
	var res string
	timeout := time.Duration(2 * time.Second)
	httpClient := http.Client{
		Timeout: timeout,
	}
	hnClient := gophernews.NewClient(httpClient)
	top100 := hnClient.GetTopStories()

	for _, id := range top100[:n] {

		story := hnClient.GetStory(id)
		res += fmt.Sprintf("Title: %s\n", story.Title)
		res += fmt.Sprintf("\tURL: %s\n", story.URL)
		res += fmt.Sprintf("\tDiscussion: %s%d\n", HNItemURLPrefix, id)

	}

	return res
}

// Get the quote via Yahoo. You should replace this method to something
// relevant to your team!
func getQuote(sym string) string {
	sym = strings.ToUpper(sym)
	url := fmt.Sprintf("http://download.finance.yahoo.com/d/quotes.csv?s=%s&f=nsl1op&e=.csv", sym)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	rows, err := csv.NewReader(resp.Body).ReadAll()
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	if len(rows) >= 1 && len(rows[0]) == 5 {
		return fmt.Sprintf("%s (%s) is trading at $%s", rows[0][0], rows[0][1], rows[0][2])
	}
	return fmt.Sprintf("unknown response format (symbol was \"%s\")", sym)
}
