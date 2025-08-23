package main

import (
	"ghtrend/pkg/app"
	"github.com/lpernett/godotenv"
)


func main(){
	_ = godotenv.Load(".env")
	app.Run()
}
