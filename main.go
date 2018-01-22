package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"flag"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/xanzy/go-gitlab"
)

var victoriaPlumProjectID = 37
var patternLibraryProjectID = 161

var projectID = patternLibraryProjectID

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

// JoinedLabels is a list of Labels
type JoinedLabels struct {
	Labels []*gitlab.Label
}

// MergeRequestData request data combined with approvals
type MergeRequestData struct {
	MergeRequest *gitlab.MergeRequest
	Approvals    *gitlab.MergeRequestApprovals
	Repository   string
}

func main() {
	var (
		httpAddr    = flag.String("http", "", "HTTP service address.")
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
	router.HandleFunc("/api/labels", app.GetLabels)

	go HandleUpdates()

	// router.HandleFunc("/", app.Index)

	log.Fatal(http.ListenAndServe(*httpAddr, router))
}

// Index does more things
func (app *App) Index(w http.ResponseWriter, r *http.Request) {
	git := app.gitlabClient

	// Set up options for the GitLab API call
	ListMergeRequestsOptions := &gitlab.ListMergeRequestsOptions{
		ListOptions: gitlab.ListOptions{
			Page:    1,
			PerPage: 100,
		},
		State: gitlab.String("opened"),
	}

	// Empty struct for storing the data on
	var templateData JoinedData

	// Get the Merge Requests for VictoriaPlum
	mergeRequestsVp, _, err := git.MergeRequests.ListMergeRequests(victoriaPlumProjectID, ListMergeRequestsOptions)
	if err != nil {
		log.Fatal(err)
	}

	// Loop over VP MRs and get their approvals
	for _, mergeRequest := range mergeRequestsVp {
		approvals, _, err := git.MergeRequests.GetMergeRequestApprovals(victoriaPlumProjectID, mergeRequest.ID)
		if err != nil {
			log.Fatal(err)
		}

		mergeRequestData := &MergeRequestData{
			mergeRequest,
			approvals,
			"victoriaplum",
		}

		templateData.MergeRequests = append(templateData.MergeRequests, mergeRequestData)

	}

	// Get the Merge Requests for the Pattern Library
	mergeRequestsPl, _, err := git.MergeRequests.ListMergeRequests(patternLibraryProjectID, ListMergeRequestsOptions)
	if err != nil {
		log.Fatal(err)
	}

	// Loop over the Pattern Library MRs and get their approvals
	for _, mergeRequest := range mergeRequestsPl {
		approvals, _, err := git.MergeRequests.GetMergeRequestApprovals(patternLibraryProjectID, mergeRequest.ID)
		if err != nil {
			log.Fatal(err)
		}

		mergeRequestData := &MergeRequestData{
			mergeRequest,
			approvals,
			"patternlibrary",
		}

		templateData.MergeRequests = append(templateData.MergeRequests, mergeRequestData)

	}

	// Broadcast back to the Websocket
	broadcast <- templateData
}

// GetLabels returns a JSON array of labels on the projects
func (app *App) GetLabels(w http.ResponseWriter, r *http.Request) {
	var labelData JoinedLabels
	git := app.gitlabClient

	patternLibraryLabels, _, err := git.Labels.ListLabels(patternLibraryProjectID)
	if err != nil {
		log.Fatal(err)
	}

	for _, label := range patternLibraryLabels {
		labelData.Labels = append(labelData.Labels, label)
	}

	victoriaPlumLabels, _, err := git.Labels.ListLabels(victoriaPlumProjectID)
	if err != nil {
		log.Fatal(err)
	}

	for _, label := range victoriaPlumLabels {
		labelData.Labels = append(labelData.Labels, label)
	}

	output, err := json.Marshal(labelData)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Fprintf(w, string(output))
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
