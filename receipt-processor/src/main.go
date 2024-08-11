package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/google/uuid"
)

const (
	success = "SUCCESS"
	failure = "FAILURE"
)

type Item struct {
	ShortDescription string  `json:"shortDescription"`
	Price            float64 `json:"price,string,omitempty"`
}

type Receipt struct {
	Id           uuid.UUID
	Retailer     string  `json:"retailer"`
	PurchaseDate string  `json:"purchaseDate"`
	PurchaseTime string  `json:"purchaseTime"`
	Total        float64 `json:"total,string,omitempty"`
	Items        []Item  `json:"items"`
}

type ReceiptProcessResponse struct {
	Id uuid.UUID `json:"id"`
}

type ReceiptPointsResponse struct {
	Points int64 `json:"points"`
}

var receiptsDB map[uuid.UUID]Receipt

var mu sync.Mutex

func applyPointsRule(rcpt *Receipt) int64 {
	retailerName := rcpt.Retailer
	currPoints := 0

	//check for alphanumeric characters in retailer name
	for _, ch := range retailerName {
		if unicode.IsDigit(ch) || unicode.IsLetter(ch) {
			currPoints += 1
		}
	}

	// fmt.Println("apha", currPoints)

	//checking total price if it contains cents

	price := strconv.FormatFloat(rcpt.Total, 'f', -1, 64)
	priceSplit := strings.Split(price, ".")

	if len(priceSplit) == 1 {
		//if total price doesnt have any fraction  part
		currPoints += 50
	}

	// fmt.Println("cents", currPoints)

	//check if total is multiple of 0.25
	if math.Mod(rcpt.Total, 0.25) == 0 {
		currPoints += 25
	}

	// fmt.Println("multiple of 0.25", currPoints)

	//checking items of the receipt, adding 5 points pers 2 items
	nosItem := (len(rcpt.Items) / 2)
	currPoints += int(math.Floor(float64(nosItem))) * 5

	// fmt.Println("items", currPoints)

	//If the trimmed length of the item description is a multiple of 3, multiply the price by 0.2 and round up to the nearest integer.
	items := rcpt.Items
	for _, item := range items {
		if len(strings.TrimSpace(item.ShortDescription))%3 == 0 {
			currPoints += int(math.Round(item.Price * 0.2))
		}

	}

	// fmt.Println("description", currPoints)

	//6 points if the day in the purchase date is odd.
	layout := "2006-01-02"
	parsedDate, err := time.Parse(layout, rcpt.PurchaseDate)

	if err != nil {
		fmt.Printf("failed to parse the date")
		// http.Error(w, "failed to parse date", http.StatusInternalServerError)
		// return
	}

	if parsedDate.Day()%2 == 1 {
		currPoints += 6
	}

	// fmt.Println("day ", currPoints)

	//10 points if the time of purchase is after 2:00pm and before 4:00pm.
	parsedTime := strings.Split(rcpt.PurchaseTime, ":")

	// fmt.Println(parsedTime)
	hr, err := strconv.ParseInt(parsedTime[0], 10, 64)

	if err != nil {
		fmt.Printf("failed to parse time %v", err)
	}

	if hr > 13 && hr < 16 {
		currPoints += 10
	}

	// fmt.Println("time", currPoints)

	return int64(currPoints)
}

func processReceipts(rcpt *Receipt, store map[uuid.UUID]Receipt) (string, error) {

	// fmt.Println("receipts db is")
	fmt.Println(*rcpt)
	mu.Lock()
	id := uuid.New()
	*&rcpt.Id = id
	fmt.Println(*rcpt)
	store[id] = *rcpt

	// keys := make([]int64, 0, len(store))

	// for k := range store {
	// 	keys = append(keys, k)
	// }

	mu.Unlock()

	return success, nil
}

func retrieveReceipt(id string, store map[uuid.UUID]Receipt) (Receipt, error) {

	parsedId, err := uuid.Parse(id)
	if err != nil {
		return Receipt{}, errors.New("failed to parse id of receipt")
	}
	rcpt, exists := store[parsedId]

	if !exists {
		return Receipt{}, errors.New("receipt with given id doesn't exist")
	}

	return rcpt, nil
}

func main() {
	fmt.Println("running the main function with reload")
	receiptsDB := make(map[uuid.UUID]Receipt)

	http.HandleFunc("/receipts/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Method is  ", r.Method)

		body, err := io.ReadAll(r.Body)

		defer r.Body.Close()

		if err != nil {
			http.Error(w, "error in reading body", http.StatusInternalServerError)
			return
		}

		fmt.Println(body)
		fmt.Println(string(body))

		passedPath := r.URL.Path

		urlParts := strings.Split(passedPath, "/")
		fmt.Println("path is ", passedPath)

		if len(urlParts) < 2 {
			fmt.Fprintf(w, "invalid url path for receipts")
			return
		}

		fmt.Println(urlParts, len(urlParts))
		for p, pval := range urlParts {
			fmt.Printf("path value is %v -> %v \n", p, pval)
		}

		fmt.Println(urlParts[0], urlParts[1], urlParts[2])
		fmt.Println(urlParts[2] == "process")

		if len(urlParts) == 3 && urlParts[2] == "process" {
			// fmt.Fprint(w, "handling receipts/process path")
			if len(body) == 0 {
				fmt.Fprintf(w, "missing body", http.StatusInternalServerError)
				return
			}
			var rcpt Receipt
			err := json.Unmarshal([]byte(body), &rcpt)
			if err != nil {
				fmt.Println("error in unmarshalling")
				fmt.Println(err)
				return
			}

			fmt.Printf("item is %+v\n", rcpt)
			_, processErr := processReceipts(&rcpt, receiptsDB)

			if processErr != nil {
				http.Error(w, "failed to process the receipt", http.StatusInternalServerError)
				return
			}
			// fmt.Fprintf(w, "processed receipt", rcpt.Id.String())

			// rcptJson, _ := json.Marshal(rcpt)
			response := ReceiptProcessResponse{
				Id: rcpt.Id,
			}

			w.Header().Set("Content-Type", "application/json")

			jsonResponse, err := json.Marshal(response)
			// fmt.Println(string(jsonResponse))

			if err != nil {
				fmt.Println(err)
				http.Error(w, "Unable to marhal json response while receipt processing", http.StatusInternalServerError)
				return
			}
			w.Write(jsonResponse)
			return
		} else if len(urlParts) == 4 && urlParts[3] == "points" {
			// fmt.Fprint(w, "handling receipts/id/points path")
			// id, _ := strconv.ParseInt(urlParts[2], 10, 64)
			id := urlParts[2]
			rcpt, err := retrieveReceipt(id, receiptsDB)

			if err != nil {
				http.Error(w, "failed to retreive points", http.StatusInternalServerError)
				// fmt.Fprintf(w, string(err.Error()))
				return
			}

			points := applyPointsRule(&rcpt)
			// msg := fmt.Sprintf("points for receipt %v are %v ", id, points)

			pts := ReceiptPointsResponse{
				Points: points,
			}

			ptsJson, err := json.Marshal(pts)
			if err != nil {
				http.Error(w, "failed to marshal json for points", http.StatusInternalServerError)
				return
			}

			w.Write(ptsJson)
			return
			// rcptJson, _ := json.Marshal(rcpt)
			// response := Res
			// fmt.Fprintf(w, string(msg))

		}
		// fmt.Fprintf(w, "receipts route")
		// fmt.Println(urlParts)
	})

	// http.HandleFunc("/receipts/process",func(w http.ResponseWriter, r *http.Request) {
	// 	s := []byte{'H','e','l','l','o'}
	// 	w.WriteHeader(http.StatusForbidden)
	// 	w.Write(s)

	// })

	// http.HandleFunc("/receipts/{id}/points", func(w http.ResponseWriter, r *http.Request) {
	// 	fmt.Fprintf(w,"receipts/id/points route")
	// })

	http.ListenAndServe(":8080", nil)
}
