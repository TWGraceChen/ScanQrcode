package main

import (
	"fmt"
	"net/http"
	"encoding/json"
    "database/sql"
    _ "github.com/lib/pq"
    "log"
    "time"
)


type ScanData struct {
    ScannedData string `json:"scannedData"`
	ScannedTime string `json:"scannedTime"`
	SelectedAct string `json:"selectedAct"`
}

var (
    db *sql.DB
)


var (
    dbHost = "127.0.0.1"
    dbPort = 5432
    dbUser = "postgres"
    dbPassword = "1234"
    dbDbname = "postgres"
)

func handleScan(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        log.Println("Invalid request method:", r.Method)
        w.Header().Set("Content-Type", "application/json")
        http.Error(w, `{"error": "Invalid request method"}`, http.StatusMethodNotAllowed)
        return
    }

    var data ScanData
    if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
        log.Printf("Failed to decode JSON: %v, error: %v", r.Body, err)
        w.Header().Set("Content-Type", "application/json")
        http.Error(w, `{"error": "Failed to decode JSON"}`, http.StatusBadRequest)
        return
    }

    layout := "2006-01-02 15:04:05"
    _, err := time.Parse(layout, data.ScannedTime)
    if err != nil {
        log.Printf("Invalid date format: %v", data.ScannedTime)
        w.Header().Set("Content-Type", "application/json")
        http.Error(w, `{"error": "Invalid date format"}`, http.StatusBadRequest)
        return
    }

    if len(data.SelectedAct) > 6 {
        log.Printf("Invalid act format: %v", data.SelectedAct)
        w.Header().Set("Content-Type", "application/json")
        http.Error(w, `{"error": "Invalid act format"}`, http.StatusBadRequest)
        return
    }

    if len(data.ScannedData) > 3 {
        log.Printf("Invalid data format: %v", data.ScannedData)
        w.Header().Set("Content-Type", "application/json")
        http.Error(w, `{"error": "Invalid data format"}`, http.StatusBadRequest)
        return
    }

    sqlstat := fmt.Sprintf(`insert into checkin (time, act, id) values ('%v', '%v', '%v')`, data.ScannedTime, data.SelectedAct, data.ScannedData)
    if _, err := db.Exec(sqlstat); err != nil {
        log.Printf("Failed to save data: %v, error: %v", sqlstat, err)
        w.Header().Set("Content-Type", "application/json")
        http.Error(w, `{"error": "Failed to save data"}`, http.StatusInternalServerError)
        return
    }

    // Respond to the client
    response := map[string]string{"message": "Data received successfully"}
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated) // 设置状态码为 201
    if err := json.NewEncoder(w).Encode(response); err != nil {
        log.Printf("Failed to encode JSON response: %v, error: %v", response, err)
        http.Error(w, `{"error": "Failed to encode JSON response"}`, http.StatusInternalServerError)
        return
    }
}

func connect() (db *sql.DB, err error) {
	db, err = sql.Open("postgres", fmt.Sprintf("host=%s port=%v user=%s password=%s dbname=%s sslmode=disable", dbHost, dbPort, dbUser, dbPassword, dbDbname))
	if err != nil {
		return db, err
	}
	return
}

func main() {
	// init database
    var err error
    db, err = connect()
	if err = db.Ping(); err != nil {
		log.Fatal("[Error] Init Database failed:", err)
	}
	defer db.Close()
	
    sqlstat := `create table if not exists checkin (time timestamp,act varchar(20),id varchar(30))`
	if _, err := db.Exec(sqlstat); err != nil {
		log.Fatal("[Error] Init Database failed:", err)
	}
	log.Println("Init Database Success.")
    
    http.Handle("/", http.FileServer(http.Dir("static")))
	http.HandleFunc("/process-scan", handleScan)
	fmt.Println("Server listening on :8080")
	http.ListenAndServe(":8080", nil)
}
