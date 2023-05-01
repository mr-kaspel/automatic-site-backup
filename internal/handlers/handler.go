package handlers

import (
	"fmt"
)

var cmd = map[string]interface{}{
	"help": help,
}

func help() {
	fmt.Println(CommandList)
}

func Listen() string {
	var text string

	fmt.Scan(&text)
	return text
}

func CommandProcessing(input string) (string, error) {
	if _, ok := cmd[input]; ok {
		cmd[input].(func())()
		return input, fmt.Errorf("total errors: %d", 0)
	} else {
		return input, nil
	}
}
