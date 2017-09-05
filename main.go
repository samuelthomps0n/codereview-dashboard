package main

import (
	"fmt"
	"github.com/gorilla/mux"
	"github.com/samuelthomps0n/go-gitlab"
	"html/template"
	"log"
	"net/http"
)

func main() {

	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/", Index)

	log.Fatal(http.ListenAndServe(":8080", router))
}

type TemplateData struct {
	*gitlab.MergeRequest
	*gitlab.MergeRequestApprovals
}

func Index(w http.ResponseWriter, r *http.Request) {

	tpl, err := template.ParseFiles("base.html", "approvals.html")
	if err != nil {
		log.Fatal(err)
	}

	git := gitlab.NewClient(nil, "")
	git.SetBaseURL("")

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
