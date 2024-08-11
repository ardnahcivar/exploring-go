package main

import (
	"bytes"
	"fmt"
	"net/http"
	"sync"
)

const (
	numGoroutines = 800
	workers       = 50
)

func main() {
	var wg sync.WaitGroup
	jobs := make(chan int, numGoroutines)

	client := &http.Client{}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go makeRequest(i, client, &wg, jobs)
	}

	for j := 0; j < numGoroutines; j++ {
		jobs <- j
	}

	close(jobs)

	wg.Wait()
}

func makeRequest(id int, client *http.Client, wg *sync.WaitGroup, jobs <-chan int) {
	hostUrl := "http://localhost:8080/receipts/process"

	payload := []byte(`{
  "retailer": "Target",
  "purchaseDate": "2022-01-01",
  "purchaseTime": "13:01",
  "items": [
    {
      "shortDescription": "Mountain Dew 12PK",
      "price": "6.49"
    },{
      "shortDescription": "Emils Cheese Pizza",
      "price": "12.25"
    },{
      "shortDescription": "Knorr Creamy Chicken",
      "price": "1.26"
    },{
      "shortDescription": "Doritos Nacho Cheese",
      "price": "3.35"
    },{
      "shortDescription": "   Klarbrunn 12-PK 12 FL OZ  ",
      "price": "12.00"
    }
  ],
  "total": "35.35"
  }`)

	defer wg.Done()

	for job := range jobs {
		req, err := http.NewRequest(http.MethodPost, hostUrl, bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		if err != nil {
			fmt.Println("Error is creating request")
			return
		}
		fmt.Println("sending req with worker", id, job)
		resp, err := client.Do(req)

		if err != nil {
			fmt.Println("Error in sending request")
			fmt.Println(err)
			return
		}

		defer resp.Body.Close()
		fmt.Println("resp Status", resp.Status, resp.StatusCode)
	}

}
