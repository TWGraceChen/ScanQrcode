package main

import (
  "fmt"
  "strings"
  "net/http"
  "io/ioutil"
)

func main() {

	url := "https://e2b5-114-36-207-158.ngrok-free.app/process-scan"
	method := "POST"
  	
	for i := 0;i< 500;i++ {
		client := &http.Client {
		}
		payload := strings.NewReader(`{"scannedData":"011","scannedTime":"2023-08-23 15:36:55","selectedAct":"一早"}`)
		req, err := http.NewRequest(method, url, payload)
	  
		if err != nil {
		  fmt.Println(err)
		  return
		}
		req.Header.Add("Content-Type", "text/plain")
	  
		res, err := client.Do(req)
		if err != nil {
		  fmt.Println(err)
		  return
		}
		defer res.Body.Close()
	  
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
		  fmt.Println(err)
		  return
		}
		fmt.Println(string(body))
	}

}