package main

import (
	"ghtrend/pkg/httpRequest"
	"log"
	"ghtrend/pkg/trending"
	"ghtrend/pkg/utils"
	"ghtrend/pkg/ui"
)


func main(){
	res, err := httpRequest.Fetch()
	if err != nil{
		log.Fatal(err)
	}
	html := string(res)

	repos , err := trending.ParseHtml(html)
	if err != nil {
		log.Fatal(err)
	}
	utils.LogJson(repos)
		
	program, err := ui.Render(repos)
	if err != nil {
		log.Fatal("err when render: ", err)
	}

	_ = program
	
}
