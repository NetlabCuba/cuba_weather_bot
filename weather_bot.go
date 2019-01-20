package main

import (
	"fmt"
	"github.com/pysrc/bs"
	tb "gopkg.in/tucnak/telebot.v2"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"time"
)

// This is a fixed url for Havana weather forecast in spanish
var BASE_URL = "https://weather.com/es-CL/tiempo/10dias/l/CUXX0003:1:CU";

// Global channels for communication.
var response_channel = make(chan *bs.Node)
var values_channel = make(chan []string)
var request_channel = make(chan string)

// GET the table containing the information we're interested in in a traversable DOM object.
func get_forecast_table() {
	response, err := http.Get(BASE_URL)
	var page string
	if err != nil {
		log.Fatal(err)
		return
	} else {
		defer response.Body.Close()
		contents, err := ioutil.ReadAll(response.Body)
		if err != nil {
			log.Fatal(err)
			return
		}
		page = string(contents)
	}

	soup := bs.Init(page)
	response_channel <- soup.SelByTag("table")[0]
}

// Parse the table and extract the information we're interested in. For now, the general description for each of the next days.
func parse_forecast_table() {
	for {
		node_tree := <-response_channel
		days := node_tree.SelByTag("tbody")[0].SelByTag("tr")
		days_values := make([]string, len(days))
		for i, day := range days {
			v := day.SelByTag("span")
			days_values[i] = fmt.Sprint(v[0].Value, " ", v[1].Value, " -> ", (*day.SelByClass("temp")[0].Attrs)["title"])
		}
		values_channel <- days_values
	}
}

//Just a helper to send a few messages at once.
func send_strarray_as_messages(bot *tb.Bot, values []string, to_whom *tb.Chat) {
	for _, v := range values {
		_, err := bot.Send(to_whom, v)
		if err != nil {
			log.Fatal(err)
		}
	}
}

// Handle the different kind of requests we accept for now.
func handle_requests() {
	var forecasts *[]string
	for {
		select {
		case request := <-request_channel:
			{
				switch request {
				// Si, nuestro bot será bilingüe 😋
				case "/update", "/actualiza":
					go get_forecast_table()
				case "/weather", "/tiempo":
					values_channel <- *forecasts
				}
			}
		case new_value := <-values_channel:
			{
				forecasts = &new_value
			}
		}
	}
}

// Create, set up and start the actual bot. This sets the "main" thread to an infinite loop of handling updates.
func start_bot() {
	// Create the bot with the token specified by @BotFather
	bot, err := tb.NewBot(tb.Settings{
		Token: "[REDACTED]",
		// You can also set custom API URL. If field is empty it equals to "https://api.telegram.org"
		//URL: "http://195.129.111.17:8012",
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
	})

	if err != nil {
		log.Fatal(err)
		return
	}

	// Set up to handle the possible incoming messages.
	bot.Handle(tb.OnText, func(message *tb.Message) {
		text := message.Text
		if strings.HasSuffix(text, bot.Me.Username) {
			query := strings.Trim(strings.Split(text, "@")[0], " @")
			request_channel <- query
			switch query {
			case "/weather", "/tiempo":
				send_strarray_as_messages(bot, <-values_channel, message.Chat)
			default:
				bot.Send(message.Chat, "🤔... no entendí")
			}
		}
	})

	bot.Start()

}
// Start everything to have initial values and all infinite loops on different threads.
func main() {
	go get_forecast_table()
	go parse_forecast_table()
	go handle_requests()
	start_bot()
}
