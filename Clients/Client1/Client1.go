package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Book struct {

	ID     int     `json:"ID,omitempty"`
	Title string		`json:"Title"`
	Author string		`json:"Author"`
	Price float64		`json:"Price"`
	Stock int64			`json:"Stock"`
}

func main() {
	
	baseURL := "http://localhost:9001/Books"

	postBook(baseURL, Book{
		Title:  "Clean Code",
		Author: "Robert C. Martin",
		Price:  29.99,
		Stock:  10,
	})

	postBook(baseURL, Book{
		Title:  "The Go Programming Language",
		Author: "Alan A. A. Donovan",
		Price:  35.50,
		Stock:  5,
	})

	postBook(baseURL, Book{
		Title:  "Designing Data-Intensive Applications",
		Author: "Martin Kleppmann",
		Price:  42.00,
		Stock:  3,
	})

	getBooks(baseURL)
}

func postBook(url string, b Book) {
	data, err := json.Marshal(b)
	if err != nil {
		fmt.Println("JSON marshal error:", err)
		return
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		fmt.Println("POST error:", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println("POST Status:", resp.Status)
	fmt.Println("POST Response:", string(body))
	fmt.Println("-----")
}

func getBooks(url string) {
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("GET error:", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println("GET Status:", resp.Status)
	fmt.Println("GET Response:", string(body))
}
