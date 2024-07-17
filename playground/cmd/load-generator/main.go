package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/kelseyhightower/envconfig"
)

type config struct {
	TargetURL    string `split_words:"true" required:"true"`
	BotTargetURL string `split_words:"true" required:"true"`
}

func main() {
	var env config
	if err := envconfig.Process("", &env); err != nil {
		log.Fatal("Failed to process env: ", err)
	}

	go makeRequest(env.TargetURL)
	makeRequest(env.BotTargetURL)
}

func makeRequest(url string) {
	for {
		resp, err := http.Get(url)
		if err != nil {
			fmt.Println("Error making GET request:", err, "URL:", url)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("Error reading response body:", err, "URL:", url)
			continue
		}

		fmt.Println("Response from API:", string(body), "URL:", url)
		resp.Body.Close()

		time.Sleep(3 * time.Second)
	}
}
