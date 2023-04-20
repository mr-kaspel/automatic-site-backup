package main

imp (
	"fmt"
	"log"
	"net/http"
	"encoding/json"
)

type Configuration struct {
	NUMBER_OF_TREADS string
}

func main() {
	// Connecting a configuration file
	configuration := Configuration{}
	err = json.Unmarshal(file, &configuration)
	failOnError(err, "Failed to decode settings into structure")
}