# Bots for Slack
Bots written in Go for Slack. Inspired by OpsDash's blog "[Build your own slack bot in Go](https://www.opsdash.com/blog/slack-bot-in-golang.html)"

Powered by Websocket and Slack RTM API the slack.go interface is very easy to write. So we are allowed to get into the fun part almost instantly.

## HackerNews Bot
One of my frequently visited sites is YC News, aka Hacker News. 

I want to get a link to the news when I ask a bot "what's the hottest news on the internet?". 
Currently the bot supports
- `news #stories` to deliver #stories news from `topstories` API call
- `top #timeout` to deliver all __best__ stories sorted by scores in top 500 API call, all within the timeout

## Technicals
The Hacker News' firebase API offer `topstories` to deliver _TOP_ stories in one single API call. 
But that doesn't mean _Best_ stories. There are more caveats about HN's v0 API:

> The v0 API is essentially a dump of our in-memory data structures. We know, what works great locally in memory isn't so hot over the network. Many of the awkward things are just the way HN works internally. Want to know the total number of comments on an article? Traverse the tree and count. Want to know the children of an item? Load the item and get their IDs, then load them. The newest page? Starts at item maxid and walks backward, keeping only the top level stories. Same for Ask, Show, etc.

Finally got time to implement this 1 year old issue.
- [x] Implement a bruteforce search that can finish 500 websocket queries with tcp round-trip >250ms each
 - tested finishing time is <= 13s on a 1 core 1Ghz VM with 500MB ram and 100Mbit internet, with 10 workers
 - it can finish within 3s if using 100 workers, average roundtrip for getting a story is 385ms each, when internet condition is excellent
- [x] Implement a caching mechanism to cache the search result, sorted by both score and time stamp
- [x] Separate the search and update with the main bot to allow multiple streams subscription
