package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"strings"
)

// rustServiceURL is where the Rust redirect engine (the hot-path service)
// is listening. In a real deployment this would come from config/env/DNS.
var rustServiceURL = getEnv("RUST_SERVICE_URL", "http://localhost:9090")

const shortCodeAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// generateCode makes a random 6-character short code.
func generateCode() (string, error) {
	b := make([]byte, 6)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(shortCodeAlphabet))))
		if err != nil {
			return "", err
		}
		b[i] = shortCodeAlphabet[n.Int64()]
	}
	return string(b), nil
}

// --- Types mirroring the Rust service's JSON contract ---

type setUrlRequest struct {
	Code        string `json:"code"`
	OriginalURL string `json:"original_url"`
}

type setUrlResponse struct {
	Success bool `json:"success"`
}

type getUrlResponse struct {
	OriginalURL string `json:"original_url"`
	Found       bool   `json:"found"`
}

type clickResponse struct {
	Success bool `json:"success"`
}

type clicksCountResponse struct {
	Count int64 `json:"count"`
}

// --- Thin client for the Rust service ---

func rustSetURL(code, originalURL string) error {
	payload, _ := json.Marshal(setUrlRequest{Code: code, OriginalURL: originalURL})
	resp, err := http.Post(rustServiceURL+"/set", "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var out setUrlResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("rust service reported failure setting url")
	}
	return nil
}

func rustGetURL(code string) (getUrlResponse, error) {
	var out getUrlResponse
	resp, err := http.Get(rustServiceURL + "/get/" + code)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&out)
	return out, err
}

func rustIncrementClick(code string) error {
	resp, err := http.Post(rustServiceURL+"/click/"+code, "application/json", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var out clickResponse
	return json.NewDecoder(resp.Body).Decode(&out)
}

func rustGetClicks(code string) (int64, error) {
	resp, err := http.Get(rustServiceURL + "/clicks/" + code)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var out clicksCountResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, err
	}
	return out.Count, nil
}

// --- HTTP handlers exposed to clients ---

type shortenRequest struct {
	URL string `json:"url"`
}

type shortenResponse struct {
	Code     string `json:"code"`
	ShortURL string `json:"short_url"`
}

func shortenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad request body", http.StatusBadRequest)
		return
	}
	var req shortenRequest
	if err := json.Unmarshal(body, &req); err != nil || req.URL == "" {
		http.Error(w, "expected JSON body: {\"url\": \"https://...\"}", http.StatusBadRequest)
		return
	}

	code, err := generateCode()
	if err != nil {
		http.Error(w, "failed generating short code", http.StatusInternalServerError)
		return
	}

	if err := rustSetURL(code, req.URL); err != nil {
		log.Printf("error calling rust service: %v", err)
		http.Error(w, "redirect engine unavailable", http.StatusBadGateway)
		return
	}

	resp := shortenResponse{
		Code:     code,
		ShortURL: fmt.Sprintf("http://localhost:8080/r/%s", code),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func redirectHandler(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimPrefix(r.URL.Path, "/r/")
	if code == "" {
		http.NotFound(w, r)
		return
	}

	result, err := rustGetURL(code)
	if err != nil {
		log.Printf("error calling rust service: %v", err)
		http.Error(w, "redirect engine unavailable", http.StatusBadGateway)
		return
	}
	if !result.Found {
		http.NotFound(w, r)
		return
	}

	// Fire-and-forget the click counter so the redirect itself stays fast.
	go func() {
		if err := rustIncrementClick(code); err != nil {
			log.Printf("error incrementing click count: %v", err)
		}
	}()

	http.Redirect(w, r, result.OriginalURL, http.StatusFound)
}

func analyticsHandler(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimPrefix(r.URL.Path, "/analytics/")
	if code == "" {
		http.NotFound(w, r)
		return
	}

	count, err := rustGetClicks(code)
	if err != nil {
		log.Printf("error calling rust service: %v", err)
		http.Error(w, "redirect engine unavailable", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clicksCountResponse{Count: count})
}

func main() {
	http.HandleFunc("/shorten", shortenHandler)
	http.HandleFunc("/r/", redirectHandler)
	http.HandleFunc("/analytics/", analyticsHandler)

	addr := ":8080"
	log.Printf("Go API listening on %s (talking to Rust engine at %s)", addr, rustServiceURL)
	log.Fatal(http.ListenAndServe(addr, nil))
}
