package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nilbot/gophernews"
	"github.com/nilbot/slackbot"
)

// URI : https://hacker-news.firebaseio.com/v0/topstories.json?print=pretty
func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: hackernews slack-bot-token\n")
		os.Exit(1)
	}

	token := os.Args[1]
	newsChannelID := slack.GetSpamChannelID(token)
	// start a websocket-based Real Time API session
	ws, id := slack.Connect(token)
	fmt.Println("hackernews ready, ^C exits")

	for {
		// read each incoming message
		m, err := slack.GetMessage(ws)
		if err != nil {
			log.Fatal(err)
		}

		// see if we're mentioned
		if m.Type == "message" &&
			strings.HasPrefix(m.Text, "<@"+id+">") {
			if m.Channel != newsChannelID {
				go func(m slack.Message) {
					m.Text = "Please kindly move to " +
						"#random first and then talk " +
						"to me again, thank you!"
					slack.PostMessage(ws, m)
				}(m)
				continue
			}
			// if so try to parse if
			parts := strings.Fields(m.Text)
			if len(parts) > 1 && parts[1] == "news" {
				// getting news
				go func(m slack.Message) {
					if len(parts) == 2 {
						m.Text = getTopNews("3")
					} else {
						m.Text = getTopNews(parts[2])
					}
					slack.PostMessage(ws, m)
				}(m) // NOTE: value copy instead of ptr ref
			} else if len(parts) > 1 && parts[1] == "top" {
				go func(m slack.Message) {
					if len(parts) == 2 {
						m.Text = topScores("5")
					} else {
						m.Text = topScores(parts[2])
					}
					slack.PostMessage(ws, m)
				}(m)
			} else if len(parts) == 3 && parts[1] == "stock" {
				// getting stock
				go func(m slack.Message) {
					m.Text = getQuote(parts[2])
					slack.PostMessage(ws, m)
				}(m)
			} else {
				// huh?
				m.Text = fmt.Sprintf("sorry, can't " +
					"serve you anything except " +
					"'news [n]', 'top [timeout in" +
					" seconds]' and 'stock " +
					"{ticker}' for now.\n")
				slack.PostMessage(ws, m)
			}
		}
	}
}

// HNItemURLPrefix : URL prefix for fetching an item on hacker news
var HNItemURLPrefix = "https://news.ycombinator.com/item?id="

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
	hnClient := gophernews.NewClient(&httpClient)
	top100, err := hnClient.GetTopStories()
	if err != nil {
		return fmt.Sprintf("HN API error: %v", err)
	}
	for _, id := range top100[:n] {

		story, err := hnClient.GetStory(id)
		if err != nil {
			return res + fmt.Sprintf("HN API error: %v", err)
		}
		res += fmt.Sprintf("Title: %s\n", story.Title)
		res += fmt.Sprintf("\tURL: %s\n", story.URL)
		res += fmt.Sprintf("\tDiscussion: %s%d\n", HNItemURLPrefix, id)

	}

	return res
}

// ScoreThreshold defines the lower bound of the score to qualify as 'top' news
var ScoreThreshold = 499

// WorkerCount defines number of goroutines for getting the news
var WorkerCount = 100

// default score 100, 500 is rare, 20 is too low
func topScores(timeoutInSeconds string) string {
	start := time.Now()
	n, err := strconv.Atoi(timeoutInSeconds)
	if err != nil {
		return fmt.Sprintf("timeout in seconds parsed error: %v", err)
	}
	if n > 60 {
		n = 60
	}

	hnClient := gophernews.NewClient(nil)
	top500, _ := hnClient.GetTopStories()

	// pipeline the workload
	in := gen(top500...)
	// fan-out the top500 ids

	chans := make([]<-chan string, WorkerCount)
	for i := range chans {
		chans[i] = make(chan string)
	}

	timeout := make(chan bool, 1)
	go func() {
		time.Sleep(time.Duration(n) * time.Second)
		timeout <- true
	}()

	for i := range chans {
		chans[i] = get(in, hnClient)
	}

	var res string

	// without time out
	// for str := range merge(chans...) {
	// 	res += str
	// }
	articles := 0
	results := merge(chans...)
	for counter := 0; counter != WorkerCount; {

		select {
		case str := <-results:
			if str != "done" {
				articles++
				res += str
			} else {
				counter++
			}
		case <-timeout:
			res += fmt.Sprintln("getting news has timed out.")
			return res
		}
	}
	return res + fmt.Sprintf("All done. I scanned %d articles, "+
		"selected %d top articles with score %d or more, using %s\n",
		len(top500), articles, ScoreThreshold, time.Since(start))
}

func gen(nums ...int) <-chan int {
	out := make(chan int)
	go func() {
		for _, n := range nums {
			out <- n
		}
		close(out)
	}()
	return out
}

func merge(cs ...<-chan string) <-chan string {
	var wg sync.WaitGroup
	out := make(chan string)

	//start a output go routine for each c in cs
	output := func(c <-chan string) {
		for n := range c {
			out <- n
		}
		wg.Done()
	}
	wg.Add(len(cs))
	for _, c := range cs {
		go output(c)
	}

	//start a goroutine to close out once all output goroutines are done.
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

func get(in <-chan int, cl *gophernews.Client) <-chan string {
	out := make(chan string)
	go func() {
		sum := time.Duration(0)
		count := 0
		for n := range in {
			start := time.Now()
			story, _ := cl.GetStory(n)
			sum += time.Since(start)
			count++
			if story.Score > ScoreThreshold {
				out <- fmt.Sprintf("Title: %s\n\t"+
					"URL: %s\n\t"+
					"Discussion: %s%d\n",
					story.Title,
					story.URL,
					HNItemURLPrefix,
					story.ID)
			}
		}
		log.Printf("worker report average roundtrip is %v\n",
			time.Duration(int64(sum)/int64(count)))
		out <- "done"
		close(out)
	}()
	return out
}

var stk = "http://download.finance.yahoo.com/d/quotes.csv?s=%s&f=nsl1op&e=.csv"

// Get the quote via Yahoo. You should replace this method to something
// relevant to your team!
func getQuote(sym string) string {
	sym = strings.ToUpper(sym)
	url := fmt.Sprintf(stk, sym)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	rows, err := csv.NewReader(resp.Body).ReadAll()
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	if len(rows) >= 1 && len(rows[0]) == 5 {
		return fmt.Sprintf("%s (%s) is trading at $%s",
			rows[0][0], rows[0][1], rows[0][2])
	}
	return fmt.Sprintf("unknown response format (symbol was \"%s\")", sym)
}
