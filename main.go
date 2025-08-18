package main

import (
	"ghtrend/pkg/ghclient"
	"log"
	"ghtrend/pkg/ui"
	"github.com/lpernett/godotenv"
)


func main(){
	_ = godotenv.Load(".env")
	
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
