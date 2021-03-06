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

// ProjectData includes the ProjectID, Project struct and Labels
type Project struct {
	Project   *gitlab.Project
	Labels    []*gitlab.Label
	ProjectID int `json:"project_id"`
}

// ProjectListData is a list of ProjectData structs
type ProjectList struct {
	Project []Project
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
	router.Handle("/app.js", fs)
	router.Handle("/styles.css", fs)
	router.HandleFunc("/webhook", app.Index)
	router.HandleFunc("/ws", HandleConnections)
	router.HandleFunc("/api/users", app.GetUsers)
	router.HandleFunc("/api/projects", app.GetProjects)

	go HandleUpdates()

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
		Scope: gitlab.String("all"),
	}

	// Empty struct for storing the data on
	var templateData JoinedData

	// Get the Merge Requests
	mergeRequests, _, err := git.MergeRequests.ListMergeRequests(ListMergeRequestsOptions)
	if err != nil {
		log.Fatal(err)
	}

	// Loop over MRs and get their approvals
	for _, mergeRequest := range mergeRequests {
		approvals, _, err := git.MergeRequests.GetMergeRequestApprovals(mergeRequest.ProjectID, mergeRequest.IID)
		if err != nil {
			log.Fatal(err)
		}

		mergeRequestData := &MergeRequestData{
			mergeRequest,
			approvals,
		}

		templateData.MergeRequests = append(templateData.MergeRequests, mergeRequestData)

	}

	// Broadcast back to the Websocket
	broadcast <- templateData
}

// GetUsers returns a JSON array of users
func (app *App) GetUsers(w http.ResponseWriter, r *http.Request) {
	git := app.gitlabClient

	ListUsersOptions := &gitlab.ListUsersOptions{
		ListOptions: gitlab.ListOptions{
			Page:    1,
			PerPage: 100,
		},
	}

	users, _, err := git.Users.ListUsers(ListUsersOptions)
	if err != nil {
		log.Fatal(err)
	}

	output, err := json.Marshal(users)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Fprintf(w, string(output))
}

// GetProjects returns a JSON array of projects
func (app *App) GetProjects(w http.ResponseWriter, r *http.Request) {
	var projectList ProjectList
	var project Project

	git := app.gitlabClient

	ListProjectOptions := &gitlab.ListProjectsOptions{
		ListOptions: gitlab.ListOptions{
			Page:    1,
			PerPage: 100,
		},
	}

	projects, _, err := git.Projects.ListProjects(ListProjectOptions)
	if err != nil {
		log.Fatal(err)
	}

	for _, apiProject := range projects {
		project.Project = apiProject
		// Get project Labels
		labels, _, err := git.Labels.ListLabels(apiProject.ID)
		if err != nil {
			log.Fatal(err)
		}

		project.Labels = labels

		// Set ProjectID
		project.ProjectID = apiProject.ID

		// Save to the list
		projectList.Project = append(projectList.Project, project)
	}

	output, err := json.Marshal(projectList)
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

		err := ws.ReadJSON(&mrd)
		if err != nil {
			log.Printf("error: %v", err)
			delete(clients, ws)
			break
		}
		// Send the newly received updates to the broadcast channel
		broadcast <- mrd
	}
}

// HandleUpdates handles updating the data being passed to all the users connected over websockets
func HandleUpdates() {
	for {
		// Grab the next updates from the broadcast channel
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
