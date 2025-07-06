package utils

import (
	"fmt"
	"encoding/json"
	"log"
)


func LogJson(thing any) { 
	data, err := json.MarshalIndent(thing, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(data))
}
