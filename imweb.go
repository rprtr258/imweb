package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/net/websocket"
)

type ID string

// incoming event, e.g. button click
type event struct {
	ID    ID     `json:"id"`
	Event string `json:"event"`
}

type elemState struct {
	Text   string
	custom any // bool for button
}

type Context[S any] struct {
	c        *websocket.Conn
	commands []command
	state    *S
	elems    *syncMap[ID, elemState]
}

type command struct {
	ID   ID     `json:"id"`
	Data string `json:"data"`
	Kind string `json:"kind"` // "ADD" | "REMOVE" | "REPLACE"
}

func (c *Context[S]) text(id ID, text string) {
	html := fmt.Sprintf(
		`<div id='%s'>%s</div>`,
		id, text,
	)
	if st, ok := c.elems.Get(id); !ok {
		c.elems.Set(id, elemState{Text: text})
		c.commands = append(c.commands, command{ID: id, Data: html, Kind: "ADD"})
	} else if st.Text != text {
		c.elems.Set(id, elemState{Text: text})
		c.commands = append(c.commands, command{ID: id, Data: html, Kind: "REPLACE"})
	}
}

func (c *Context[S]) button(id ID) bool {
	html := fmt.Sprintf(
		`<button id='%[1]s' onclick='window.IMWEB_notify({id: "%[1]s", event: "clicked"})'>%[1]s</button>`,
		id,
	)
	if st, ok := c.elems.Get(id); !ok {
		c.elems.Set(id, elemState{Text: string(id), custom: false})
		c.commands = append(c.commands, command{ID: id, Data: html, Kind: "ADD"})
	} else if st.custom.(bool) {
		st.custom = false
		c.elems.Set(id, st)
		return true
	}
	return false
}

func run[S any](init S, appFn func(*Context[S])) error {
	log.SetFlags(log.Lshortfile)
	const fps = 20
	const refreshInterval = 1000 / fps * time.Millisecond
	log.Println("running on", ":4040")
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		// TODO: serve static
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!DOCTYPE html>
<html lang="en">
<head>

<title>Aboba</title>
<script type="text/javascript">
window.onload = () => {
	const body = document.body;

	if (!window["WebSocket"]) {
		body.innerHTML = "<b>Your browser does not support WebSockets.</b>";
		return;
	}

	let ws;
	connect = () => {
		document.body.innerHTML = "";
		ws = new WebSocket("ws://" + document.location.host + "/ws");
		ws.onclose = (e) => {
			console.log('Socket is closed. Reconnect will be attempted in 1 second.', e.reason);
			setTimeout(() => {
				connect();
			}, 1000);
		};
		ws.onerror = (err) => {
			console.error('Socket encountered error: ', err, 'Closing socket');
			ws.close();
		};
		ws.onmessage = (e) => {
			const message = JSON.parse(e.data);
			switch (message.kind) {
			case "ADD":
				body.innerHTML += message.data;
				break;
			case "REMOVE":
				document.getElementById(message.id).remove();
				break;
			case "REPLACE":
				document.getElementById(message.id).outerHTML = message.data;
				break;
			}
		};
	};

	connect();
	window.IMWEB_notify = (msg) => {
		ws.send(JSON.stringify(msg));
	};
};
</script>

</head>
<body></body>
</html>`))
	})
	http.Handle("/ws", websocket.Handler(func(c *websocket.Conn) {
		var elems syncMap[ID, elemState]
		queue := make(chan event, 100)
		defer close(queue)
		// read events
		go func() {
			for {
				var m event
				if err := websocket.JSON.Receive(c, &m); err != nil {
					log.Println("read event:", err)
					return
				}
				queue <- m
				log.Printf("received event: %#v\n", m)
			}
		}()
		// send updates
		last := time.Now()
		for {
			// wait for next frame
			now := time.Now()
			if elapsed := now.Sub(last); elapsed < refreshInterval {
				last = now
				time.Sleep(refreshInterval - elapsed)
			}
			last = now

			// process events
			notProcessedEvents := []event{}
			updatedIDs := map[ID]struct{}{}
		EVENTS_LOOP:
			for {
				select {
				case e := <-queue:
					if _, ok := updatedIDs[e.ID]; ok {
						// event on updated element, process event on next frame
						notProcessedEvents = append(notProcessedEvents, e)
						continue EVENTS_LOOP
					}

					updatedIDs[e.ID] = struct{}{}
					if e.Event == "clicked" {
						st := elems.MustGet(e.ID)
						st.custom = true
						elems.Set(e.ID, st)
					}
				default:
					break EVENTS_LOOP
				}
			}
			for _, e := range notProcessedEvents {
				queue <- e
			}

			// update
			ctx := Context[S]{c, nil, &init, &elems}
			appFn(&ctx)
			for _, cmd := range ctx.commands {
				if err := websocket.JSON.Send(c, cmd); err != nil {
					if strings.Contains(err.Error(), "broken pipe") {
						log.Println("conn closed")
						return
					}
					log.Println("write:", err)
				}
			}
		}
	}))
	server := &http.Server{
		Addr:    ":4040",
		Handler: http.DefaultServeMux,
	}
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-ch
		if err := server.Shutdown(context.Background()); err != nil {
			log.Println("shutdown:", err.Error())
		}
	}()
	return server.ListenAndServe()
}
