package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

const (
	EVENT_TASK_ALERT   = 1
	EVENT_TASKS_UPDATE = 2
)

// New event messages are broadcast to all registered client connection channels
type ClientChan chan *Event

type Event struct {
	Message interface{}
	Type    int
}

type StreamController struct {
	// Events are pushed to this channel by the main events-gathering routine
	Message chan *Event

	// New client connections
	NewClients chan ClientChan

	// Closed client connections
	ClosedClients chan ClientChan

	// Total client connections
	TotalClients map[ClientChan]bool
}

func NewStreamController() (sc *StreamController) {
	sc = &StreamController{
		Message:       make(chan *Event),
		NewClients:    make(chan ClientChan),
		ClosedClients: make(chan ClientChan),
		TotalClients:  make(map[ClientChan]bool),
	}

	go sc.listen()

	return
}

// It Listens all incoming requests from clients.
// Handles addition and removal of clients and broadcast messages to clients.
func (sc *StreamController) listen() {
	for {
		select {
		// Add new available client
		case client := <-sc.NewClients:
			sc.TotalClients[client] = true
			log.Info().Msgf("Client added. %d registered clients", len(sc.TotalClients))

		// Remove closed client
		case client := <-sc.ClosedClients:
			delete(sc.TotalClients, client)
			close(client)
			log.Info().Msgf("Removed client. %d registered clients", len(sc.TotalClients))

		// Broadcast message to client
		case eventMsg := <-sc.Message:
			for clientMessageChan := range sc.TotalClients {
				clientMessageChan <- eventMsg
			}
		}
	}
}

func (sc *StreamController) ServeHTTP() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Initialize client channel
		clientChan := make(ClientChan)

		// Send new connection to event server
		sc.NewClients <- clientChan

		defer func() {
			// Send closed connection to event server
			sc.ClosedClients <- clientChan
		}()

		c.Set("clientChan", clientChan)

		c.Next()
	}
}

func StreamHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("Transfer-Encoding", "chunked")
		c.Next()
	}
}
