package main

import (
	"ghtrend/pkg/httpRequest"
	"fmt"
	"log"
	"ghtrend/pkg/trending"
	"ghtrend/pkg/utils"
	"ghtrend/pkg/ui"
)


func main(){
	res, err := httpRequest.Fetch("https://github.com/trending")
	if err != nil{
		log.Fatal(err)
	}
	html := string(res)

	repos , err := trending.ParseHtml(html)
	if err != nil {
		log.Fatal(err)
	}
	utils.LogJson(repos)
		
	hi, err := httpRequest.GetRawGithubReadmeFile("snap-stanford", "Biomni")
	if err != nil {
		fmt.Println("haldsf")
	}
	fmt.Println (hi)
	
	program, err := ui.Render(repos)
	if err != nil {
		log.Fatal("err when render: ", err)
	}

	_ = program
	
}
