package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"

	"flag"

	"github.com/gorilla/mux"
	"github.com/xanzy/go-gitlab"
)

type App struct {
	gitlabClient *gitlab.Client
}

func main() {
	var (
		httpAddr    = flag.String("http", ":80", "HTTP service address.")
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

type TemplateData struct {
	*gitlab.MergeRequest
	*gitlab.MergeRequestApprovals
}

func (app *App) Index(w http.ResponseWriter, r *http.Request) {
	git := app.gitlabClient

	tpl, err := template.ParseFiles("base.html", "approvals.html")
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

	mergeRequests, _, err := git.MergeRequests.ListMergeRequests(172, ListMergeRequestsOptions)

	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(mergeRequests)

	tpl.Execute(w, mergeRequests)

	for _, mergeRequest := range mergeRequests {
		approvals, _, err := git.MergeRequests.GetMergeRequestApprovals(172, mergeRequest.ID)
		if err != nil {
			log.Fatal(err)
		}

		tpl.ExecuteTemplate(w, "approvals", approvals)

		fmt.Println(approvals)
	}

}
