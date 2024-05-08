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
	TargetURL string `split_words:"true" required:"true"`
}

func main() {
	var env config
	if err := envconfig.Process("", &env); err != nil {
		log.Fatal("Failed to process env: ", err)
	}

	for {
		resp, err := http.Get(env.TargetURL)
		if err != nil {
			fmt.Println("Error making GET request:", err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("Error reading response body:", err)
			continue
		}

		fmt.Println("Response from API:", string(body))
		resp.Body.Close()

		time.Sleep(2 * time.Second)
	}
}
