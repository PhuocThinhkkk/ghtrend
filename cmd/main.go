package main

import (
	"fmt"
	"ghtrend/pkg/httpRequest"
	"log"
	"ghtrend/pkg/trending"
	"ghtrend/pkg/utils"
	"ghtrend/pkg/ui"
)


func main(){
	fmt.Println("hi mom")
	res, err := httpRequest.Fetch()
	if err != nil{
		log.Fatal(err)
	}
	html := string(res)

	repos , err := trending.ParseHtml(html)
	if err != nil {
		log.Fatal(err)
	}
	// utils.LogJson(repos)
	// will cleaning this up later

	var utilsRepos []utils.Repo
	for _, r := range repos { // repos is []trending.Repo
		utilsRepos = append(utilsRepos, utils.Repo{
			Index:       r.Index,
			Name:        r.Name,
			Url:         r.Url,
			Description: r.Description,
			Language:    r.Language,
			Stars:       r.Stars,
			Forks:       r.Forks,
		})
	}

		
	program, err := ui.Render(utilsRepos)
	if err != nil {
		log.Fatal("err when render: ", err)
	}

	_ = program
	
}
