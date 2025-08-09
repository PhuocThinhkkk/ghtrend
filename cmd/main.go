package main

import (
	"ghtrend/pkg/ghclient"
	"log"
	"ghtrend/pkg/ui"
)


func main(){
	
	repos, err := ghclient.GetAllTrendingRepos()
	if err != nil {
		log.Fatal(err)
	}
	program, err := ui.Render(repos)
	if err != nil {
		log.Fatal("err when render: ", err)
	}

	_ = program
	
}
