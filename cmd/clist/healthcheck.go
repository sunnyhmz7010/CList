package main

import (
	"errors"
	"net/http"
	"time"
)

func runHealthcheck() error {
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Get("http://127.0.0.1:8080/health/ready")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return errors.New("CList 未就绪")
	}
	return nil
}
