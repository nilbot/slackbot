package main

import (
	"container/heap"
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

var cache = struct {
	m map[int]gophernews.Story
	sync.RWMutex
}{m: make(map[int]gophernews.Story)}

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
	go backgroundFetchNews()
	for {
		// read each incoming message
		m, err := slack.GetMessage(ws)
		if err != nil {
			log.Fatal(err)
		}

		// see if we're mentioned
		if m.Type == "message" &&
			strings.HasPrefix(m.Text, "<@"+id+">") {
			if _, ok := newsChannelID[m.Channel]; !ok {
				go func(m slack.Message) {
					m.Text = "Please kindly move to " +
						"#random or #test-chamber " +
						"first and then talk " +
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
						m.Text = randomNews("3")
					} else {
						m.Text = randomNews(parts[2])
					}
					slack.PostMessage(ws, m)
				}(m) // NOTE: value copy instead of ptr ref
			} else if len(parts) > 1 && parts[1] == "top" {
				go func(m slack.Message) {
					if len(parts) == 2 {
						m.Text = topNews("5")
					} else {
						m.Text = topNews(parts[2])
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

func randomNews(topN string) string {
	n, err := strconv.Atoi(topN)
	if err != nil {
		return fmt.Sprintf("top n parsed error: %v", err)
	}
	if n > len(cache.m) {
		n = len(cache.m)
	}
	res := "Delivering random news...\n"
	count := 1
	for k, v := range cache.m {
		if count > n {
			break
		}
		res += fmt.Sprintf("Title: %s\n", v.Title)
		res += fmt.Sprintf("\tURL: %s\n", v.URL)
		res += fmt.Sprintf("\tDiscussion: %s%d\n", HNItemURLPrefix, k)
		count++
	}

	return res
}

// ScoreThreshold defines the lower bound of the score to qualify as 'top' news
var ScoreThreshold = 500

// WorkerCount defines number of goroutines for getting the news
var WorkerCount = 100

// default score 100, 500 is rare, 20 is too low
func topNews(kstr string) string {
	k, err := strconv.Atoi(kstr)
	if err != nil {
		return fmt.Sprintf("top k parsed error: %v", err)
	}
	res := fmt.Sprintf("Delivering top %d news...\n", k)
	rq := make(RankQueue, len(cache.m))
	i := 0
	for _, story := range cache.m {
		rq[i] = &Rank{
			Index: story.ID,
			Title: story.Title,
			Score: story.Score,
			URL:   story.URL,
		}
		i++
	}
	heap.Init(&rq)
	min := 1 << 20
	max := -1
	for j := 0; j < k; j++ {
		rank := rq.Pop().(*Rank)
		if min > rank.Score {
			min = rank.Score
		}
		if max < rank.Score {
			max = rank.Score
		}
		res += fmt.Sprintf("Title: %s\n\t"+
			"URL: %s\n\t"+
			"Discussion: %s%d\n",
			rank.Title,
			rank.URL,
			HNItemURLPrefix,
			rank.Index)
	}
	return res + fmt.Sprintf("All done. I scanned %d articles, "+
		"selected %d top articles sorted with score(min:%d, max:%d).\n",
		len(cache.m), k, min, max)
}
func backgroundFetchNews() {
	threshold := time.Duration(15) * time.Minute //15minutes
	start := time.Now()
	iter := 0
	for {
		diff := time.Since(start)
		if diff > threshold {
			start = time.Now()
			iter++
			log.Println("iteration: ", iter)
			hnClient := gophernews.NewClient(nil)
			hnNewsIDs, _ := hnClient.GetTopStories()

			// pipeline the workload
			in := gen(hnNewsIDs...)
			// fan-out the hnNewsIDs ids

			chans := make([]<-chan *gophernews.Story, WorkerCount)
			for i := range chans {
				chans[i] = make(chan *gophernews.Story)
			}

			for i := range chans {
				chans[i] = get(in, hnClient)
			}

			results := merge(chans...)
			for counter := 0; counter != WorkerCount; {
				str := <-results
				if str != nil {
					cache.m[str.ID] = *str
				} else {
					counter++
				}
			}
		}
	}
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

func merge(cs ...<-chan *gophernews.Story) <-chan *gophernews.Story {
	var wg sync.WaitGroup
	out := make(chan *gophernews.Story)

	//start a output go routine for each c in cs
	output := func(c <-chan *gophernews.Story) {
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

func get(in <-chan int, cl *gophernews.Client) <-chan *gophernews.Story {
	out := make(chan *gophernews.Story)
	go func() {
		sum := time.Duration(0)
		count := 0
		for n := range in {
			start := time.Now()
			story, _ := cl.GetStory(n)
			sum += time.Since(start)
			count++
			if story.Score >= ScoreThreshold {
				out <- &story
			}
		}
		log.Printf("worker report average roundtrip is %v\n",
			time.Duration(int64(sum)/int64(count)))
		out <- nil
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

// Rank is rankable story
type Rank struct {
	URL   string
	Title string
	Score int
	Index int
}

// RankQueue is priority queue ranked by descending scores
type RankQueue []*Rank

func (rq RankQueue) Len() int { return len(rq) }

func (rq RankQueue) Less(i, j int) bool {
	return rq[i].Score > rq[j].Score
}

func (rq RankQueue) Swap(i, j int) {
	rq[i], rq[j] = rq[j], rq[i]
	rq[i].Index = i
	rq[j].Index = j
}

// Push insert a story
func (rq *RankQueue) Push(x interface{}) {
	n := len(*rq)
	rank := x.(*Rank)
	rank.Index = n
	*rq = append(*rq, rank)
}

// Pop returns the top score
func (rq *RankQueue) Pop() interface{} {
	old := *rq
	n := len(old)
	rank := old[n-1]
	rank.Index = -1 // for safety
	*rq = old[0 : n-1]
	return rank
}

// Update the score of a rank in the queue.
func (rq *RankQueue) Update(rank *Rank, score int) {
	rank.Score = score
	heap.Fix(rq, rank.Index)
}
