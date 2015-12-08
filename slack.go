package slack

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"

	"golang.org/x/net/websocket"
)

// ResponseRtmStart and responseSelf structures represent the response of the
// Slack API rtm.start. Only some fields are included. The rest are ignored by
// json.Unmarshal.
type ResponseRtmStart struct {
	Ok    bool         `json:"ok"`
	Error string       `json:"error"`
	URL   string       `json:"url"`
	Self  responseSelf `json:"self"`
}

type responseSelf struct {
	ID string `json:"id"`
}

// Start does a rtm.start, and returns a websocket URL and user ID. The
// websocket URL can be used to initiate an RTM session.
func Start(token string) (wsurl, id string, err error) {
	url := fmt.Sprintf("https://slack.com/api/rtm.start?token=%s", token)
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	if resp.StatusCode != 200 {
		err = fmt.Errorf("API request failed with code %d", resp.StatusCode)
		return
	}

	var respObj ResponseRtmStart
	err = json.NewDecoder(resp.Body).Decode(&respObj)
	if err != nil {
		return
	}

	if !respObj.Ok {
		err = fmt.Errorf("Slack error: %s", respObj.Error)
		return
	}

	wsurl = respObj.URL
	id = respObj.Self.ID
	return
}

// Message are the messages read off and written into the websocket. Since this
// struct serves as both read and write, we include the "ID" field which is
// required only for writing.
type Message struct {
	ID      uint64 `json:"id"`
	Type    string `json:"type"`
	Channel string `json:"channel"`
	Text    string `json:"text"`
}

// GetMessage wraps websocket and return a Message
func GetMessage(ws *websocket.Conn) (m Message, err error) {
	err = websocket.JSON.Receive(ws, &m)
	return
}

var counter uint64

// PostMessage post a Message via websocket
func PostMessage(ws *websocket.Conn, m Message) error {
	m.ID = atomic.AddUint64(&counter, 1)
	return websocket.JSON.Send(ws, m)
}

// Connect starts a websocket-based Real Time API session and return the websocket
// and the ID of the (bot-)user whom the token belongs to.
func Connect(token string) (*websocket.Conn, string) {
	wsurl, id, err := Start(token)
	if err != nil {
		log.Fatal(err)
	}

	ws, err := websocket.Dial(wsurl, "", "https://api.slack.com/")
	if err != nil {
		log.Fatal(err)
	}

	return ws, id
}

// ResponseChannelList constains the response of channel.list call
type ResponseChannelList struct {
	Ok       bool      `json:"ok"`
	Error    string    `json:"error"`
	Channels []Channel `json:"channels"`
}

// Channel contains information about a team channel
type Channel struct {
	ID                 string         `json:"id"`
	Name               string         `json:"name"`
	Creator            string         `json:"creator"`
	LastRead           float64        `json:"last_read"`
	UnreadCount        int            `json:"unread_count"`
	UnreadCountDisplay int            `json:"unread_count_display"`
	IsMember           bool           `json:"is_member"`
	IsChannel          bool           `json:"is_channel"`
	IsArchived         bool           `json:"is_archived"`
	IsGeneral          bool           `json:"is_general"`
	Members            []string       `json:"members"`
	Purpose            ChannelPurpose `json:"purpose"`
	Topic              ChannelTopic   `json:"topic"`
	Latest             Message        `json:"latest"`
}

// Caption describes topic or purpose
type Caption struct {
	Value   string `json:"value"`
	Creator string `json:"creator"`
	LastSet int    `json:"last_set"`
}

// ChannelTopic is channel's Topic
type ChannelTopic Caption

// ChannelPurpose is channel's purpose
type ChannelPurpose Caption

// IM contains information about a direct message channel
type IM struct {
	ID            string `json:"id"`
	IsIM          bool   `json:"is_im"`
	User          string `json:"user"`
	Created       int    `json:"created"`
	IsUserDeleted bool   `json:"is_user_deleted"`
}

// GetChannelList returns a slice of Channel
func GetChannelList(token string) (result []Channel, err error) {
	url := fmt.Sprintf("https://slack.com/api/rtm.start?token=%s", token)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		err = fmt.Errorf("API request failed with code %d", resp.StatusCode)
		return nil, err
	}

	var respObj ResponseChannelList
	err = json.NewDecoder(resp.Body).Decode(&respObj)
	if err != nil {
		return nil, err
	}

	if !respObj.Ok {
		err = fmt.Errorf("Slack error: %s", respObj.Error)
		return nil, err
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// GetGeneralChannelID returns the ID of #general channel
func GetGeneralChannelID(token string) (ID string) {
	list, e := GetChannelList(token)
	if e != nil {
		log.Fatal(e)
	}
	for _, c := range list {
		if c.IsGeneral {
			ID = c.ID
			break
		}
	}
	return
}

// GetSpamChannelID returns the ID of #random channel
func GetSpamChannelID(token string) (ID string) {
	list, e := GetChannelList(token)
	if e != nil {
		log.Fatal(e)
	}
	for _, c := range list {
		if c.Name == "random" {
			ID = c.ID
			break
		}
	}
	return
}
