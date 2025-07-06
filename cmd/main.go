package main

import (
	"fmt"
	"ghtrend/pkg/httpRequest"
	"log"
	"ghtrend/pkg/trending"
	//"ghtrend/pkg/utils"
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
		
}
