package handlers

import (
	"fmt"
	"net/http"
)

var logClients = make(map[chan string]bool)
var logBroadcast = make(chan string, 100)

func init() {
	go func() {
		for msg := range logBroadcast {
			for client := range logClients {
				select {
				case client <- msg:
				default:
				}
			}
		}
	}()
}

func BroadcastLog(msg string) {
	select {
	case logBroadcast <- msg:
	default:
	}
}

func Logs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	client := make(chan string, 10)
	logClients[client] = true
	defer func() {
		delete(logClients, client)
		close(client)
	}()

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming no soportado", http.StatusInternalServerError)
		return
	}

	for {
		select {
		case msg := <-client:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}