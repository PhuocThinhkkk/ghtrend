package main

import (
	"github.com/PhuocThinhkkk/ghtrend/cmd"
	"github.com/lpernett/godotenv"
)


func main(){
	_ = godotenv.Load(".env")
	cmd.Execute()
}
