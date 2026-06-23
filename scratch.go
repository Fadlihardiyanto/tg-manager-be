package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
)

func main() {
	serverKey := "asal_saja_random"
	url := "https://app.sandbox.midtrans.com/snap/v1/transactions"
	req, _ := http.NewRequest("POST", url, bytes.NewBufferString("{}"))
	
	credentials := serverKey + ":"
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(credentials)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Println("StatusCode:", resp.StatusCode)
	fmt.Println("Body:", string(body))
}
