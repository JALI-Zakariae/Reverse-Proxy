package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
)
type Book struct{
	ID int     		 	`json:"ID"`
	Title string		`json:"Title"`
	Author string		`json:"Author"`
	Price float64		`json:"Price"`
	Stock int64			`json:"Stock"`
}
var BooksMu  = struct{
		Books []Book
		mu sync.Mutex
}{}



var NextID  = struct {
	mu sync.Mutex
	nextID int
}{
	nextID: 1,
}


func main(){
	http.HandleFunc("/Books",BooksHandler)
	log.Println("Book server running on :9002")
	log.Fatal(http.ListenAndServe(":9001",nil))
}
func BooksHandler(w http.ResponseWriter,r *http.Request){
	switch r.Method{
	case http.MethodGet:
		HandleGetUsers(w)
	case http.MethodPost:
		HandlePostUsers(w,r)
	default:
		http.Error(w,"Method not allowed yet",http.StatusMethodNotAllowed)
	}
}
func HandleGetUsers(w http.ResponseWriter){
	w.Header().Set("Content-Type","application/json")
	BooksMu.mu.Lock()
	json.NewEncoder(w).Encode(BooksMu.Books)
	BooksMu.mu.Unlock()
}
func HandlePostUsers(w http.ResponseWriter,r *http.Request){
	w.Header().Set("Content-Type", "application/json")
	var book Book
	if err:=json.NewDecoder(r.Body).Decode(&book);err!=nil{
		http.Error(w,"Invalid JSON",http.StatusBadRequest)
		return
	}
	NextID.mu.Lock()
	book.ID = NextID.nextID
	NextID.nextID++
	NextID.mu.Unlock()
	BooksMu.mu.Lock()
	BooksMu.Books = append(BooksMu.Books,book)
	BooksMu.mu.Unlock()
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(book)
}