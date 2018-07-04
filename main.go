package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

const patience time.Duration = time.Second * 1

var ssePrefix = []byte("data: ")
var sseSuffix = []byte("\n\n")

type Broker struct {
	caches         map[string]*Cache
	newClients     chan Subscription
	closingClients chan Subscription
	clients        map[string]map[chan []byte]bool
}

type Subscription struct {
	channel chan []byte
	stream  string
}

func NewServer() (broker *Broker) {
	broker = &Broker{
		caches:         make(map[string]*Cache),
		newClients:     make(chan Subscription),
		closingClients: make(chan Subscription),
		clients:        make(map[string]map[chan []byte]bool),
	}
	go broker.listen()
	
	return
}

func (broker *Broker) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	stream := strings.ToLower(req.RequestURI[1:])

	if stream == "" {
		rw.Write([]byte("HOME"))
		return
	}
	if strings.HasPrefix(stream, "s/") {
		stream = stream[2:]
	} else if strings.HasPrefix(stream, "v/") {
		data, _ := ioutil.ReadFile("index.html")
		rw.Write(data)
		return
	} else {
		data, _ := ioutil.ReadFile(stream)
		rw.Write(data)
		return
	}
	if req.Method == "GET" {
		flusher, ok := rw.(http.Flusher)

		if !ok {
			http.Error(rw, "Streaming unsupported!", http.StatusInternalServerError)
			return
		}

		rw.Header().Set("Content-Type", "text/event-stream")
		rw.Header().Set("Cache-Control", "no-cache")
		rw.Header().Set("Connection", "keep-alive")
		rw.Header().Set("Access-Control-Allow-Origin", "*")

		// Each connection registers its own message channel with the Broker's connections registry
		sub := Subscription{
			channel: make(chan []byte),
			stream:  stream,
		}

		// Signal the broker that we have a new connection
		broker.newClients <- sub

		// Remove this client from the map of connected clients
		defer func() {
			broker.closingClients <- sub
		}()

		// Listen to connection close and un-register messageChan
		notify := rw.(http.CloseNotifier).CloseNotify()

		if cache, ok := broker.caches[stream]; ok {
			cache.get(rw)
			flusher.Flush()
		}

		for {
			select {
			case <-notify:
				return
			default:

				rw.Write(ssePrefix)
				rw.Write(<-sub.channel)
				rw.Write(sseSuffix)

				// Flush the data immediatly instead of buffering it for later.
				flusher.Flush()
			}
		}
	} else if req.Method == "POST" {
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			http.Error(rw, "Could not read POST body!", http.StatusInternalServerError)
		}
		defer req.Body.Close()

		broker.notify(stream, body)
	}
}

func (broker *Broker) notify(stream string, data []byte) {
	var cache *Cache
	if _, ok := broker.caches[stream]; !ok {
		cache = NewCache(stream, broker)
		cache.add(data)
		broker.caches[stream] = cache
	}
	if clientMap, ok := broker.clients[stream]; ok {
		for clientMessageChan := range clientMap {
			select {
			case clientMessageChan <- data:
			case <-time.After(patience):
				log.Print("Skipping client.")
			}
		}
	}
	if cache != nil {
		go cache.listen(broker)
	}
}

func (broker *Broker) listen() {
	for {
		select {
		case s := <-broker.newClients:

			// A new client has connected
			if _, ok := broker.clients[s.stream]; !ok {
				broker.clients[s.stream] = make(map[chan []byte]bool)
			}
			broker.clients[s.stream][s.channel] = true

			broker.notify("log", []byte(fmt.Sprintf("Client added to stream %s. %d registered clients on this stream.", s.stream, len(broker.clients[s.stream]))))

		case s := <-broker.closingClients:

			// A client has disconnected
			if len(broker.clients[s.stream]) == 0 {
				delete(broker.clients, s.stream)
			} else {
				delete(broker.clients[s.stream], s.channel)
			}
			broker.notify("log", []byte(fmt.Sprintf("Removed client from stream %s.", s.stream)))
		}
	}
}

func main() {
	broker := NewServer()
	go exampleSender(broker)
	log.Fatal("HTTP server error: ", http.ListenAndServe("0.0.0.0:80", broker))

}

func exampleSender(broker *Broker) {
	var i uint64
	for {
		time.Sleep(time.Millisecond * 500)
		eventString := fmt.Sprint(i)
		broker.notify("count", []byte(eventString))
		i++
	}
}
