package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"github.com/spf13/viper"
)

var (
	db          *sql.DB
	dbHost      string
	dbPort      string
	dbUser      string
	dbPassword  string
	dbDbname    string
	dbSslMode   string
	serviceName string
	port        string
	queryParam  string
)

// --- Structs ---
type SettingField struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type SearchFilter struct {
	Key     string   `json:"key"`
	Label   string   `json:"label"`
	Options []string `json:"options"`
}

type RegisterPayload struct {
	Name       *string                `json:"name,omitempty"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
	IsDisabled *bool                  `json:"is_disabled,omitempty"`
}

type ScanPayload struct {
	ScannedData string `json:"scannedData"`
	ScannedTime string `json:"scannedTime"`
	SelectedAct string `json:"selectedAct"`
}

type SearchPayload struct {
	SearchName string            `json:"name"`
	SearchAct  string            `json:"act"`
	Attributes map[string]string `json:"attributes"`
}

type DetailSearch struct {
	Id          string                 `json:"id"`
	Name        string                 `json:"name"`
	Attributes  map[string]interface{} `json:"attributes"`
	Act         string                 `json:"act"`
	CheckinTime string                 `json:"checkinTime"`
}

type ReportPayload struct {
	SearchName    string            `json:"name"`
	SearchGeTimes string            `json:"ge"`
	SearchLeTimes string            `json:"le"`
	Attributes    map[string]string `json:"attributes"`
}

type DetailReport struct {
	Id         string                 `json:"id"`
	Name       string                 `json:"name"`
	Attributes map[string]interface{} `json:"attributes"`
	Times      int                    `json:"times"`
}

type SettingsPayload struct {
	Acts         []string       `json:"acts"`
	SearchFields []SettingField `json:"search_fields"`
}

type TemplateScan struct {
	Title string
	Acts  []string
}

type TemplateSearch struct {
	Title        string
	SearchFields []SearchFilter
	Acts         []string
}

type TemplateReport struct {
	Title        string
	SearchFields []SearchFilter
	Acts         []string
	Times        []int
}

type TemplateAdmin struct {
	Title string
}

// --- Utils ---

func connect() (db *sql.DB, err error) {
	connStr := fmt.Sprintf("host=%s port=%v user=%s password=%s dbname=%s sslmode=%s", dbHost, dbPort, dbUser, dbPassword, dbDbname, dbSslMode)
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		return db, err
	}
	return
}

func generateRandomID(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func readConfig() (err error) {
	if _, err := os.Stat("config.yaml"); err == nil {
		v := viper.New()
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		err = v.ReadInConfig()
		if err != nil {
			return err
		}

		serviceName = v.GetString("service.name")
		port = v.GetString("service.port")
		queryParam = v.GetString("service.param")
		dbHost = v.GetString("db.host")
		dbPort = v.GetString("db.port")
		dbUser = v.GetString("db.user")
		dbPassword = v.GetString("db.password")
		dbDbname = v.GetString("db.database")
		dbSslMode = v.GetString("db.sslmode")

		log.Println("Read config file Success.")
	} else {
		serviceName = os.Getenv("name")
		port = os.Getenv("port")
		queryParam = os.Getenv("param")
		dbHost = os.Getenv("dbhost")
		dbPort = os.Getenv("dbport")
		dbUser = os.Getenv("dbuser")
		dbPassword = os.Getenv("dbpassword")
		dbDbname = os.Getenv("database")
		dbSslMode = os.Getenv("sslmode")
		log.Println("Read env Success.")
	}
	return nil
}

func getSettings() SettingsPayload {
	var payload SettingsPayload
	var actsJson []byte
	err := db.QueryRow("SELECT value FROM settings WHERE key = 'acts'").Scan(&actsJson)
	if err == nil {
		json.Unmarshal(actsJson, &payload.Acts)
	}

	var searchFieldsJson []byte
	err = db.QueryRow("SELECT value FROM settings WHERE key = 'search_fields'").Scan(&searchFieldsJson)
	if err == nil {
		json.Unmarshal(searchFieldsJson, &payload.SearchFields)
	}

	return payload
}

func getDistinctAttributes(fields []SettingField) []SearchFilter {
	var filters []SearchFilter
	for _, field := range fields {
		filter := SearchFilter{Key: field.Key, Label: field.Label, Options: []string{"*"}}
		
		query := fmt.Sprintf("SELECT DISTINCT attributes->>'%s' FROM register WHERE is_disabled = false AND attributes->>'%s' IS NOT NULL ORDER BY attributes->>'%s'", field.Key, field.Key, field.Key)
		rows, err := db.Query(query)
		if err != nil {
			log.Printf("Failed to get distinct %s: %v", field.Key, err)
			filters = append(filters, filter)
			continue
		}
		
		for rows.Next() {
			var val string
			if err := rows.Scan(&val); err == nil && val != "" {
				filter.Options = append(filter.Options, val)
			}
		}
		rows.Close()
		filters = append(filters, filter)
	}
	return filters
}

// --- Handlers ---
func handleRegisterAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var payload RegisterPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if payload.Name == nil || *payload.Name == "" {
			http.Error(w, "Name is required", http.StatusBadRequest)
			return
		}

		id := generateRandomID(8)
		if payload.Attributes == nil {
			payload.Attributes = make(map[string]interface{})
		}
		attrsBytes, _ := json.Marshal(payload.Attributes)

		_, err := db.Exec(`
			INSERT INTO register (id, name, attributes, is_disabled) 
			VALUES ($1, $2, $3, false)
		`, id, *payload.Name, string(attrsBytes))

		if err != nil {
			log.Printf("Failed to register: %v", err)
			http.Error(w, "Failed to register", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": id})
		return
	}

	if r.Method == http.MethodPut {
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "Missing id", http.StatusBadRequest)
			return
		}

		var payload RegisterPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		var setClauses []string
		var args []interface{}
		argId := 1

		if payload.Name != nil {
			setClauses = append(setClauses, fmt.Sprintf("name = $%d", argId))
			args = append(args, *payload.Name)
			argId++
		}

		if payload.Attributes != nil {
			attrsBytes, _ := json.Marshal(payload.Attributes)
			setClauses = append(setClauses, fmt.Sprintf("attributes = $%d", argId))
			args = append(args, string(attrsBytes))
			argId++
		}

		if payload.IsDisabled != nil {
			setClauses = append(setClauses, fmt.Sprintf("is_disabled = $%d", argId))
			args = append(args, *payload.IsDisabled)
			argId++
		}

		if len(setClauses) == 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"message": "No updates provided"})
			return
		}

		setClauses = append(setClauses, "updated_at = CURRENT_TIMESTAMP")
		
		query := fmt.Sprintf("UPDATE register SET %s WHERE id = $%d", strings.Join(setClauses, ", "), argId)
		args = append(args, id)

		_, err := db.Exec(query, args...)
		if err != nil {
			log.Printf("Failed to update register: %v", err)
			http.Error(w, "Failed to update register", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "Updated successfully"})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func handleCheckInAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var payload ScanPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		layout := "2006-01-02 15:04:05"
		parsedTime, err := time.Parse(layout, payload.ScannedTime)
		if err != nil {
			http.Error(w, "Invalid date format", http.StatusBadRequest)
			return
		}

		loc, _ := time.LoadLocation("Asia/Taipei")
		parsedTimeInSysZone := parsedTime.In(loc)
		ScannedTimeWithTimeZone := parsedTimeInSysZone.Format(layout)

		id := ""
		if queryParam != "" {
			parsedURL, err := url.Parse(payload.ScannedData)
			if err != nil {
				http.Error(w, "Invalid data format", http.StatusBadRequest)
				return
			}
			queryParams := parsedURL.Query()
			if params, exists := queryParams[queryParam]; exists {
				id = params[0]
			} else {
				http.Error(w, "Invalid data format", http.StatusBadRequest)
				return
			}
		} else {
			id = payload.ScannedData
		}

		var name string
		err = db.QueryRow("SELECT name FROM register WHERE id = $1", id).Scan(&name)
		if err == sql.ErrNoRows {
			http.Error(w, "Not qualified for check in", http.StatusBadRequest)
			return
		} else if err != nil {
			http.Error(w, "Failed to get register info", http.StatusInternalServerError)
			return
		}

		_, err = db.Exec(`
			INSERT INTO checkin (time, act, id) 
			VALUES ($1, $2, $3)
		`, ScannedTimeWithTimeZone, payload.SelectedAct, id)
		
		if err != nil {
			log.Printf("Failed to check in:%v", err)
			http.Error(w, "Failed to check in", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"name": name})
		return 
	}
	
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)	
}

func handleSearchAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var payload SearchPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		var condition []string
		var args []interface{}
		argId := 1

		condition = append(condition, "b.is_disabled = false")

		if payload.SearchName != "" {
			condition = append(condition, fmt.Sprintf("b.name LIKE $%d", argId))
			args = append(args, "%"+payload.SearchName+"%")
			argId++
		}
		if payload.SearchAct != "" && payload.SearchAct != "*" {
			condition = append(condition, fmt.Sprintf("a.act = $%d", argId))
			args = append(args, payload.SearchAct)
			argId++
		}

		for k, v := range payload.Attributes {
			if v != "" && v != "*" {
				condition = append(condition, fmt.Sprintf("b.attributes->>'%s' = $%d", k, argId))
				args = append(args, v)
				argId++
			}
		}

		where := ""
		if len(condition) > 0 {
			where = " WHERE " + strings.Join(condition, " AND ")
		}

		query := `SELECT b.id, b.name, b.attributes, a.act, min(a."time") as checkinTime
		FROM checkin a 
		INNER JOIN register b ON a.id = b.id ` + where + `
		GROUP BY b.id, b.name, b.attributes, a.act`

		var searchResult []DetailSearch
		rows, err := db.Query(query, args...)
		if err != nil {
			log.Printf("Failed to search result, query:%v, error: %v", query, err)
			http.Error(w, "Failed to retrieve search results", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var record DetailSearch
			var attrsBytes []byte
			var checkinTime time.Time

			err = rows.Scan(&record.Id, &record.Name, &attrsBytes, &record.Act, &checkinTime)
			if err != nil {
				log.Printf("Failed to scan result, error: %v", err)
				http.Error(w, "Failed to scan result", http.StatusInternalServerError)
				return
			}
			json.Unmarshal(attrsBytes, &record.Attributes)
			record.CheckinTime = checkinTime.Format("2006-01-02 15:04:05")

			searchResult = append(searchResult, record)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(searchResult)
		return
	}
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func handleReportAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var payload ReportPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		var condition []string
		var args []interface{}
		argId := 1

		condition = append(condition, "b.is_disabled = false")

		if payload.SearchName != "" {
			condition = append(condition, fmt.Sprintf("b.name LIKE $%d", argId))
			args = append(args, "%"+payload.SearchName+"%")
			argId++
		}
		for k, v := range payload.Attributes {
			if v != "" && v != "*" {
				condition = append(condition, fmt.Sprintf("b.attributes->>'%s' = $%d", k, argId))
				args = append(args, v)
				argId++
			}
		}

		where := ""
		if len(condition) > 0 {
			where = " WHERE " + strings.Join(condition, " AND ")
		}

		var timeCondition []string
		if payload.SearchGeTimes != "" {
			timeCondition = append(timeCondition, fmt.Sprintf("times >= %v", payload.SearchGeTimes))
		}
		if payload.SearchLeTimes != "" {
			timeCondition = append(timeCondition, fmt.Sprintf("times <= %v", payload.SearchLeTimes))
		}
		
		timeswhere := ""
		if len(timeCondition) > 0 {
			timeswhere = " WHERE " + strings.Join(timeCondition, " AND ")
		}

		baseSelect := "SELECT id, count(distinct act) as times FROM checkin GROUP BY id"
		joinedSelect := fmt.Sprintf(`SELECT b.id, b.name, b.attributes, COALESCE(a.times, 0) as times 
			FROM register b 
			LEFT JOIN (%s) a ON b.id = a.id
			%s`, baseSelect, where)
			
		query := fmt.Sprintf("SELECT * FROM (%s) c %s", joinedSelect, timeswhere)

		var searchResult []DetailReport
		rows, err := db.Query(query, args...)
		if err != nil {
			log.Printf("Failed to search result, query:%v, error: %v", query, err)
			http.Error(w, "Failed to retrieve search results", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var record DetailReport
			var attrsBytes []byte

			err = rows.Scan(&record.Id, &record.Name, &attrsBytes, &record.Times)
			if err != nil {
				log.Printf("Failed to scan result, error: %v", err)
				http.Error(w, "Failed to scan result", http.StatusInternalServerError)
				return
			}
			json.Unmarshal(attrsBytes, &record.Attributes)

			searchResult = append(searchResult, record)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(searchResult)
		return
	}
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func handleSettingsAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(getSettings())
		return
	}

	if r.Method == http.MethodPost {
		var payload SettingsPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		actsBytes, _ := json.Marshal(payload.Acts)
		searchFieldsBytes, _ := json.Marshal(payload.SearchFields)

		_, err1 := db.Exec("INSERT INTO settings (key, value) VALUES ('acts', $1) ON CONFLICT (key) DO UPDATE SET value = $1", string(actsBytes))
		_, err2 := db.Exec("INSERT INTO settings (key, value) VALUES ('search_fields', $1) ON CONFLICT (key) DO UPDATE SET value = $1", string(searchFieldsBytes))

		if err1 != nil || err2 != nil {
			http.Error(w, "Failed to update settings", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "Settings updated successfully"})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func handleScan(w http.ResponseWriter, r *http.Request) {
	settings := getSettings()
	data := TemplateScan{
		Title: serviceName,
		Acts:  settings.Acts,
	}

	tmpl, err := template.ParseFiles("static/scan.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err = tmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	settings := getSettings()

	actOptions := []string{"*"}
	actOptions = append(actOptions, settings.Acts...)

	templatedata := TemplateSearch{
		Title:        serviceName,
		SearchFields: getDistinctAttributes(settings.SearchFields),
		Acts:         actOptions,
	}

	tmpl, err := template.ParseFiles("static/search.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err = tmpl.Execute(w, templatedata); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleReport(w http.ResponseWriter, r *http.Request) {
	settings := getSettings()

	var timeseq []int
	for i := 0; i <= len(settings.Acts); i++ {
		timeseq = append(timeseq, i)
	}

	templatedata := TemplateReport{
		Title:        serviceName,
		SearchFields: getDistinctAttributes(settings.SearchFields),
		Acts:         settings.Acts,
		Times:        timeseq,
	}

	tmpl, err := template.ParseFiles("static/report.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err = tmpl.Execute(w, templatedata); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleAdmin(w http.ResponseWriter, r *http.Request) {
	data := TemplateAdmin{
		Title: serviceName,
	}

	tmpl, err := template.ParseFiles("static/admin.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err = tmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func main() {
	if err := readConfig(); err != nil {
		log.Fatal("[Error] Loading config failed: ", err)
	}

	var err error
	if db, err = connect(); err != nil {
		log.Fatal("[Error] Connect Database failed:", err)
	}
	if err = db.Ping(); err != nil {
		log.Fatal("[Error] Init Database failed:", err)
	}
	defer db.Close()

	rand.Seed(time.Now().UnixNano())

	// Database Initialization
	createSettings := `
	CREATE TABLE IF NOT EXISTS settings (
		key VARCHAR(50) PRIMARY KEY,
		value JSONB
	);
	`
	if _, err := db.Exec(createSettings); err != nil {
		log.Fatal("[Error] Create settings table failed:", err)
	}

	createRegister := `
	CREATE TABLE IF NOT EXISTS register (
		id VARCHAR(20) PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		attributes JSONB,
		is_disabled BOOLEAN DEFAULT FALSE,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`
	if _, err := db.Exec(createRegister); err != nil {
		log.Fatal("[Error] Create register table failed:", err)
	}

	createCheckin := `
	CREATE TABLE IF NOT EXISTS checkin (
		time TIMESTAMP,
		act VARCHAR(20),
		id VARCHAR(20)
	);
	`
	if _, err := db.Exec(createCheckin); err != nil {
		log.Fatal("[Error] Create checkin table failed:", err)
	}

	log.Println("Init Database Success.")

	// 靜態檔案路由
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// 前端頁面路由 (負責解析模板並回傳 HTML)
	http.HandleFunc("/", handleScan)
	http.HandleFunc("/search", handleSearch)
	http.HandleFunc("/report", handleReport)
	http.HandleFunc("/admin", handleAdmin)

	// 後端資料 API 路由 (負責處理邏輯並回傳 JSON)
	http.HandleFunc("/api/register", handleRegisterAPI)
	http.HandleFunc("/api/checkin", handleCheckInAPI)
	http.HandleFunc("/api/search", handleSearchAPI)
	http.HandleFunc("/api/report", handleReportAPI)
	http.HandleFunc("/api/settings", handleSettingsAPI)




	fmt.Println("Server listening on :" + port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Printf("server error:%v", err)
	}
}
