package main

import (
	"fmt"
	"net/http"
	"encoding/json"
    "database/sql"
    _ "github.com/lib/pq"
    "log"
    "time"
    "github.com/spf13/viper"
    "os"
    "html/template"
)


type ScanData struct {
    ScannedData string `json:"scannedData"`
	ScannedTime string `json:"scannedTime"`
	SelectedAct string `json:"selectedAct"`
}

type PageData struct {
	Title string
	Acts  []string // 用于存储场次选项的切片
}

var (
    db *sql.DB
    dbHost string
    dbPort string
    dbUser string
    dbPassword string
    dbDbname string
    serviceName string
    options []string
    port string
)

func handleStatic(w http.ResponseWriter, r *http.Request) {
	// 定义要替换的标题
	data := PageData{
		Title: serviceName,
		Acts:  options, // 添加场次选项
	}

	// 解析HTML模板文件
	tmpl, err := template.ParseFiles("static/index.html")
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

func connect() (db *sql.DB, err error) {
	db, err = sql.Open("postgres", fmt.Sprintf("host=%s port=%v user=%s password=%s dbname=%s sslmode=disable", dbHost, dbPort, dbUser, dbPassword, dbDbname))
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
        options = v.GetStringSlice("service.options")
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
	
    sqlstat := `create table if not exists checkin (time timestamp,act varchar(20),id varchar(30))`
	if _, err := db.Exec(sqlstat); err != nil {
		log.Fatal("[Error] Init Database failed:", err)
	}
	log.Println("Init Database Success.")
    
	http.HandleFunc("/", handleStatic)
    http.HandleFunc("/process-scan", handleScan)
	fmt.Println("Server listening on :"+port)
	http.ListenAndServe(":"+port, nil)
}
