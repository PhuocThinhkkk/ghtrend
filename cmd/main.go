package main

import (
	"ghtrend/pkg/httpRequest"
    "fmt"
	"log"
	"ghtrend/pkg/ui"
)


func main(){
	
	repos, err := httpRequest.GetAllTrendingRepos()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(repos[0].ReadMe)
	program, err := ui.Render(repos)
	if err != nil {
		log.Fatal("err when render: ", err)
	}

	_ = program
	
}
