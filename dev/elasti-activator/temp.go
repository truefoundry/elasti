package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

func main() {
	// Create a context with a 2-second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel() // Ensure the cancel function is called to release resources

	// Perform the HTTP request with timeout
	err := doRequest(ctx)
	if err != nil {
		fmt.Printf("Request failed: %v\n", err)
	} else {
		fmt.Println("Request succeeded")
	}
}

func doRequest(ctx context.Context) error {
	// Create an HTTP request
	req, err := http.NewRequest("GET", "https://fakestoreapi.com/products/1", nil)
	if err != nil {
		return err
	}

	// Attach the context to the request
	req = req.WithContext(ctx)

	// Perform the HTTP request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		select {
		case <-ctx.Done():
			// Context timeout or cancellation occurred
			return fmt.Errorf("request canceled or timed out: %w", ctx.Err())
		default:
			// Other error occurred
			return fmt.Errorf("request failed: %w", err)
		}
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Print the response body
	fmt.Println("Response:", string(body))
	return nil
}

