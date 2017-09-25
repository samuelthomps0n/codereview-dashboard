package main

import (

	"log"
	"net/http"

	"flag"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/xanzy/go-gitlab"
)

var projectID = 37

// var projectID = 170
var clients = make(map[*websocket.Conn]bool) // connected clients
var broadcast = make(chan JoinedData)        // broadcast channel

var upgrader = websocket.Upgrader{}

// App exports some stuff
type App struct {
	gitlabClient *gitlab.Client
}

// JoinedData is a list of MergeRequestData
type JoinedData struct {
	MergeRequests []*MergeRequestData
}

// MergeRequestData request data combined with approvals
type MergeRequestData struct {
	MergeRequest *gitlab.MergeRequest
	Approvals    *gitlab.MergeRequestApprovals
}

func main() {
	var (
		httpAddr    = flag.String("http", ":8081", "HTTP service address.")
		baseURL     = flag.String("baseurl", "", "Base URL gitlab endpoint")
		gitlabToken = flag.String("gitlab-token", "", "The access token for gitlab")
	)
	flag.Parse()

	log.Println("Starting server...")
	log.Printf("HTTP service listening on %s", *httpAddr)

	git := gitlab.NewClient(nil, *gitlabToken)
	git.SetBaseURL(*baseURL)

	app := &App{git}

	router := mux.NewRouter().StrictSlash(true)
	fs := http.FileServer(http.Dir("./public"))
	router.Handle("/", fs)
	router.HandleFunc("/webhook", app.Index)
	router.HandleFunc("/ws", HandleConnections)

	go HandleUpdates()

	router.HandleFunc("/", app.Index)

	log.Fatal(http.ListenAndServe(*httpAddr, router))
}

// Index does more things
func (app *App) Index(w http.ResponseWriter, r *http.Request) {
	git := app.gitlabClient

	ListMergeRequestsOptions := &gitlab.ListMergeRequestsOptions{
		ListOptions: gitlab.ListOptions{
			Page:    1,
			PerPage: 100,
		},
		State: gitlab.String("opened"),
	}

	mergeRequests, _, err := git.MergeRequests.ListMergeRequests(projectID, ListMergeRequestsOptions)
	if err != nil {
		log.Fatal(err)
	}

	var templateData JoinedData

	for _, mergeRequest := range mergeRequests {
		approvals, _, err := git.MergeRequests.GetMergeRequestApprovals(projectID, mergeRequest.ID)
		if err != nil {
			log.Fatal(err)
		}

		mergeRequestData := &MergeRequestData{
			mergeRequest,
			approvals,
		}

		templateData.MergeRequests = append(templateData.MergeRequests, mergeRequestData)

	}

	broadcast <- templateData

}

// HandleConnections handles the Websocket connection correctly
func HandleConnections(w http.ResponseWriter, r *http.Request) {
	// Upgrade initial GET request to a websocket
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	// Make sure we close the connection when the function returns
	defer ws.Close()

	// Register our new client
	clients[ws] = true

	for {
		var mrd JoinedData
		// Read in a new message as JSON and map it to a Message object
		err := ws.ReadJSON(&mrd)
		if err != nil {
			log.Printf("error: %v", err)
			delete(clients, ws)
			break
		}
		// Send the newly received message to the broadcast channel
		broadcast <- mrd
	}
}

// HandleUpdates handles updating the data being passed to all the users connected over websockets
func HandleUpdates() {
	for {
		// Grab the next message from the broadcast channel
		mrd := <-broadcast
		// Send it out to every client that is currently connected

		for client := range clients {
			err := client.WriteJSON(mrd)
			if err != nil {
				log.Printf("error: %v", err)
				client.Close()
				delete(clients, client)
			}
		}
	}
}
