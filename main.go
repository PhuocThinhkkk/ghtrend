package main

import (
	"ghtrend/cmd"
	"github.com/lpernett/godotenv"
)


func main(){
	_ = godotenv.Load(".env")
	cmd.Execute()
}
