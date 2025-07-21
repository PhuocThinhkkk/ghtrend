package main

import (
	"ghtrend/pkg/httpRequest"
	"log"
	"ghtrend/pkg/ui"
)


func main(){
	
	repos, err := httpRequest.GetAllTrendingRepos()
	if err != nil {
		log.Fatal(err)
	}
	program, err := ui.Render(repos)
	if err != nil {
		log.Fatal("err when render: ", err)
	}

	_ = program
	
}
