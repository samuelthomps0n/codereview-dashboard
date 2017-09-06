package main

import (
	"html/template"
	"log"
	"net/http"

	"flag"

	"github.com/gorilla/mux"
	"github.com/xanzy/go-gitlab"
)

// App exports some stuff
type App struct {
	gitlabClient *gitlab.Client
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
	router.HandleFunc("/", app.Index)

	log.Fatal(http.ListenAndServe(*httpAddr, router))
}

// MergeRequestData request data combined with approvals
type MergeRequestData struct {
	MergeRequest *gitlab.MergeRequest
	Approvals    *gitlab.MergeRequestApprovals
}

// Index does more things
func (app *App) Index(w http.ResponseWriter, r *http.Request) {
	git := app.gitlabClient

	tpl, err := template.ParseFiles("base.html")
	if err != nil {
		log.Fatal(err)
	}

	ListMergeRequestsOptions := &gitlab.ListMergeRequestsOptions{
		ListOptions: gitlab.ListOptions{
			Page:    1,
			PerPage: 100,
		},
		State: gitlab.String("opened"),
	}

	mergeRequests, _, err := git.MergeRequests.ListMergeRequests(161, ListMergeRequestsOptions)

	if err != nil {
		log.Fatal(err)
	}

	var templateData []*MergeRequestData

	for _, mergeRequest := range mergeRequests {
		approvals, _, err := git.MergeRequests.GetMergeRequestApprovals(161, mergeRequest.ID)
		if err != nil {
			log.Fatal(err)
		}

		mergeRequestData := &MergeRequestData{
			mergeRequest,
			approvals,
		}

		templateData = append(templateData, mergeRequestData)

	}

	tpl.Execute(w, templateData)

}
