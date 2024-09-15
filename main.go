package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
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
	serviceName string
	actoptions  []string
	port        string
)

type TemplateScan struct {
	Title string
	Acts  []string
}

type PostScan struct {
	ScannedData string `json:"scannedData"`
	ScannedTime string `json:"scannedTime"`
	SelectedAct string `json:"selectedAct"`
}

type TemplateSearch struct {
	Title  string
	Church []IdName
	Group  []string
	Team   []string
	Acts   []string
}

type PostSearch struct {
	SearchChurch string `json:"church"`
	SearchName   string `json:"name"`
	SearchGroup  string `json:"group"`
	SearchTeam   string `json:"team"`
	SearchAct    string `json:"act"`
}

type DetailSearch struct {
	Church      string
	Group       string
	Team        string
	Name        string
	Act         string
	CheckinTime string
}

type TemplateReport struct {
	Title  string
	Church []IdName
	Group  []string
	Team   []string
	Times  []int
}

type PostReport struct {
	SearchChurch  string `json:"church"`
	SearchGroup   string `json:"group"`
	SearchTeam    string `json:"team"`
	SearchName    string `json:"name"`
	SearchGeTimes string `json:"ge"`
	SearchLeTimes string `json:"le"`
}

type DetailReport struct {
	Church string
	Group  string
	Team   string
	Name   string
	Times  string
}

type IdName struct {
	Id   int
	Name string
}

func getChurchList() (list []IdName) {
	rows, err := db.Query("SELECT id,name FROM church")
	if err != nil {
		log.Printf("Failed to search church list,error: %v", err)
	}
	for rows.Next() {
		var church IdName
		err = rows.Scan(&church.Id, &church.Name)
		if err != nil {
			log.Printf("Failed to search church list,error: %v", err)
		}
		list = append(list, church)
	}
	return
}

func getGroupList() (list []string) {
	rows, err := db.Query("select group_name from register group by group_name order by group_name")
	if err != nil {
		log.Printf("Failed to search group list,error: %v", err)
	}
	for rows.Next() {
		var group string
		err = rows.Scan(&group)
		if err != nil {
			log.Printf("Failed to search group list,error: %v", err)
		}
		list = append(list, group)

	}
	return
}

func getTeamList() (list []string) {
	rows, err := db.Query("select team from register group by team order by length(team),team")
	if err != nil {
		log.Printf("Failed to search team list,error: %v", err)
	}
	for rows.Next() {
		var team string
		err = rows.Scan(&team)
		if err != nil {
			log.Printf("Failed to team group list,error: %v", err)
		}
		list = append(list, team)

	}
	return
}

func handleScan(w http.ResponseWriter, r *http.Request) {
	// 定义要替换的标题
	data := TemplateScan{
		Title: serviceName,
		Acts:  actoptions, // 添加场次选项
	}

	// 解析HTML模板文件
	tmpl, err := template.ParseFiles("static/scan.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 使用模板替换{{.Title}}
	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleCheckIn(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		log.Println("Invalid request method:", r.Method)
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error": "Invalid request method"}`, http.StatusMethodNotAllowed)
		return
	}

	var data PostScan
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		log.Printf("Failed to decode JSON: %v, error: %v", r.Body, err)
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error": "Failed to decode JSON"}`, http.StatusBadRequest)
		return
	}

	layout := "2006-01-02 15:04:05"
	parsedTime, err := time.Parse(layout, data.ScannedTime)
	if err != nil {
		log.Printf("Invalid date format: %v", data.ScannedTime)
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error": "Invalid date format"}`, http.StatusBadRequest)
		return
	}

	// 将解析后的时间转换到系统时区
	loc, _ := time.LoadLocation("Asia/Taipei")
	parsedTimeInSysZone := parsedTime.In(loc)

	// 将转换后的时间重新格式化为原始字符串格式
	ScannedTimeZone := parsedTimeInSysZone.Format(layout)

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

	sqlstat := fmt.Sprintf(`insert into checkin (time, act, id) values ('%v', '%v', '%v')`, ScannedTimeZone, data.SelectedAct, data.ScannedData)
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

func handleSearch(w http.ResponseWriter, r *http.Request) {
	// search result
	if r.Method == http.MethodPost {
		var postData PostSearch
		if err := json.NewDecoder(r.Body).Decode(&postData); err != nil {
			log.Printf("Failed to decode JSON: %v, error: %v", r.Body, err)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// prepare sql statement
		var condition []string
		if postData.SearchChurch != "-1" {
			condition = append(condition, fmt.Sprintf("b.church_id = %v", postData.SearchChurch))
		}
		if postData.SearchGroup != "*" {
			condition = append(condition, fmt.Sprintf("b.group_name = '%v'", postData.SearchGroup))
		}
		if postData.SearchTeam != "*" {
			condition = append(condition, fmt.Sprintf("b.team = '%v'", postData.SearchTeam))
		}
		if postData.SearchName != "" {
			condition = append(condition, fmt.Sprintf("b.name like '%%%v%%'", postData.SearchName))
		}
		if postData.SearchAct != "*" {
			condition = append(condition, fmt.Sprintf("a.act = '%v'", postData.SearchAct))
		}

		var where string
		if len(condition) > 0 {
			where = " where " + strings.Join(condition, " and ")
		}

		selectstat := `SELECT c.name, b.group_name, b.team, b.name, a.act, min(a."time")
		FROM checkin a 
		INNER JOIN register b ON a.id = b.id 
		INNER JOIN church c ON b.church_id = c.id `

		groupby := " group by c.name, b.group_name, b.team, b.name, a.act"

		query := selectstat + where + groupby

		// query table
		var searchResult []DetailSearch
		rows, err := db.Query(query) // You can adjust this with WHERE conditions based on postData
		if err != nil {
			log.Printf("Failed to search result,query:%v,error: %v", query, err)
			http.Error(w, "Failed to retrieve search results", http.StatusInternalServerError)
			return
		}

		for rows.Next() {
			var record DetailSearch
			var checkinTime string // Temporary variable to hold the raw time string

			err = rows.Scan(&record.Church, &record.Group, &record.Team, &record.Name, &record.Act, &checkinTime)
			if err != nil {
				log.Printf("Failed to search result,error: %v", err)
				http.Error(w, "Failed to scan result", http.StatusInternalServerError)
				return
			}

			// Parse and format the time
			t, err := time.Parse(time.RFC3339, checkinTime)
			if err != nil {
				log.Printf("Failed to parse time, error: %v", err)
				http.Error(w, "Failed to parse time", http.StatusInternalServerError)
				return
			}
			record.CheckinTime = t.Format("2006-01-02 15:04:05")

			searchResult = append(searchResult, record)
		}

		// Return JSON response with search results
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(searchResult)
		return
	}

	// search church list
	churchOptions := []IdName{{Id: -1, Name: "*"}}
	churchOptions = append(churchOptions, getChurchList()...)

	// search group list
	groupGroups := []string{"*"}
	groupGroups = append(groupGroups, getGroupList()...)

	// search team list
	teamGroups := []string{"*"}
	teamGroups = append(teamGroups, getTeamList()...)

	// acts list
	actOptions := []string{"*"}
	actOptions = append(actOptions, actoptions...)

	templatedata := TemplateSearch{
		Title:  serviceName,
		Church: churchOptions,
		Group:  groupGroups,
		Team:   teamGroups,
		Acts:   actOptions,
	}

	// 解析HTML模板文件
	tmpl, err := template.ParseFiles("static/search.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 使用模板替换{{.Title}}
	err = tmpl.Execute(w, templatedata)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleReport(w http.ResponseWriter, r *http.Request) {
	// search result
	if r.Method == http.MethodPost {
		var postData PostReport
		if err := json.NewDecoder(r.Body).Decode(&postData); err != nil {
			log.Printf("Failed to decode JSON: %v, error: %v", r.Body, err)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// prepare sql statement
		var condition []string
		var timecondition []string
		if postData.SearchChurch != "-1" {
			condition = append(condition, fmt.Sprintf("b.church_id = %v", postData.SearchChurch))
		}
		if postData.SearchGroup != "*" {
			condition = append(condition, fmt.Sprintf("b.group_name = '%v'", postData.SearchGroup))
		}
		if postData.SearchTeam != "*" {
			condition = append(condition, fmt.Sprintf("b.team = '%v'", postData.SearchTeam))
		}
		if postData.SearchName != "" {
			condition = append(condition, fmt.Sprintf("b.name like '%%%v%%'", postData.SearchName))
		}
		if postData.SearchGeTimes != "" {
			timecondition = append(timecondition, fmt.Sprintf("times >= %v", postData.SearchGeTimes))
		}
		if postData.SearchLeTimes != "" {
			timecondition = append(timecondition, fmt.Sprintf("times <= %v", postData.SearchLeTimes))
		}

		var where string
		if len(condition) > 0 {
			where = " where " + strings.Join(condition, " and ")
		}

		var timeswhere string
		if len(timecondition) > 0 {
			timeswhere = " where " + strings.Join(timecondition, " and ")
		}

		selectstat := "select id,count(distinct act) as times from checkin group by id"
		selectstat = fmt.Sprintf(`select c.name,b.group_name,b.team,b.name,case when a.times is null then 0 else a.times end as times 
				  From register b 
		          INNER JOIN church c on b.church_id = c.id
				  LEFT JOIN (%v) a on b.id = a.id`, selectstat)
		query := fmt.Sprintf("select * from (%v %v) %v", selectstat, where, timeswhere)

		// query table
		var searchResult []DetailReport
		rows, err := db.Query(query) // You can adjust this with WHERE conditions based on postData
		if err != nil {
			log.Printf("Failed to search result,query:%v,error: %v", query, err)
			http.Error(w, "Failed to retrieve search results", http.StatusInternalServerError)
			return
		}

		for rows.Next() {
			var record DetailReport

			err = rows.Scan(&record.Church, &record.Group, &record.Team, &record.Name, &record.Times)
			if err != nil {
				log.Printf("Failed to search result,error: %v", err)
				http.Error(w, "Failed to scan result", http.StatusInternalServerError)
				return
			}

			searchResult = append(searchResult, record)
		}

		// Return JSON response with search results
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(searchResult)
		return
	}

	// search church list
	churchOptions := []IdName{{Id: -1, Name: "*"}}
	churchOptions = append(churchOptions, getChurchList()...)

	// search group list
	groupGroups := []string{"*"}
	groupGroups = append(groupGroups, getGroupList()...)

	// search team list
	teamGroups := []string{"*"}
	teamGroups = append(teamGroups, getTeamList()...)

	//time seq
	var timeseq []int
	for i := 0; i <= len(actoptions); i++ {
		timeseq = append(timeseq, i)
	}

	templatedata := TemplateReport{
		Title:  serviceName,
		Church: churchOptions,
		Group:  groupGroups,
		Team:   teamGroups,
		Times:  timeseq,
	}

	// 解析HTML模板文件
	tmpl, err := template.ParseFiles("static/report.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 使用模板替换{{.Title}}
	err = tmpl.Execute(w, templatedata)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func connect() (db *sql.DB, err error) {
	db, err = sql.Open("postgres", fmt.Sprintf("host=%s port=%v user=%s password=%s dbname=%s", dbHost, dbPort, dbUser, dbPassword, dbDbname))
	if err != nil {
		return db, err
	}
	return
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
		actoptions = v.GetStringSlice("service.options")
		dbHost = v.GetString("db.host")
		dbPort = v.GetString("db.port")
		dbUser = v.GetString("db.user")
		dbPassword = v.GetString("db.password")
		dbDbname = v.GetString("db.database")

		log.Println("Read config file Success.")
	} else {
		serviceName = os.Getenv("name")
		port = os.Getenv("port")
		dbHost = os.Getenv("dbhost")
		dbPort = os.Getenv("dbport")
		dbUser = os.Getenv("dbuser")
		dbPassword = os.Getenv("dbpassword")
		dbDbname = os.Getenv("database")
		log.Println("Read env Success.")
	}

	return nil
}

func main() {
	// read config file
	var err error
	err = readConfig()
	if err != nil {
		log.Fatal("[Error] Loading config failed: ", err)
	}

	// init database
	db, err = connect()
	if err = db.Ping(); err != nil {
		log.Fatal("[Error] Init Database failed:", err)
	}
	defer db.Close()

	sqlstat := `create table if not exists checkin (time timestamp,act varchar(20),id char(3))`
	if _, err := db.Exec(sqlstat); err != nil {
		log.Fatal("[Error] Init Database failed:", err)
	}

	log.Println("Init Database Success.")

	// 設置靜態文件目錄
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))
	http.HandleFunc("/", handleScan)
	http.HandleFunc("/checkin", handleCheckIn)
	http.HandleFunc("/search", handleSearch)
	http.HandleFunc("/report", handleReport)
	fmt.Println("Server listening on :" + port)
	err = http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Printf("server error:%v", err)
	}
}
