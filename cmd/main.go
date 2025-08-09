package main

import (
	"ghtrend/pkg/ghclient"
	"ghtrend/pkg/utils"
	"log"
	"ghtrend/pkg/ui"
)


func main(){
	
	repos, err := ghclient.GetAllTrendingRepos()
	if err != nil {
		log.Fatal(err)
	}
	utils.LogJson(repos[0].RootInfor)
	program, err := ui.Render(repos)
	if err != nil {
		log.Fatal("err when render: ", err)
	}

	_ = program
	
}
