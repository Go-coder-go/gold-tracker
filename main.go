package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
)

const (
	stateFile = "state.json"

	// 🔔 ALERT CONFIG (22K gold price)
	targetGoldPrice22K = 12700.0
	resetBuffer        = 200.0
)

var (
	goldAPIKey        = os.Getenv("GOLD_API_KEY")
	pushoverAppToken  = os.Getenv("PUSHOVER_APP_TOKEN")
	pushoverUserKey   = os.Getenv("PUSHOVER_USER_KEY")
)

type State struct {
	AlertTriggered bool `json:"alert_triggered"`
}

type GoldAPIResponse struct {
	PricePerOunce float64 `json:"price"`
}

func main() {
	log.Println("🚀 Gold Alert Job Started")
	run()
}

func run() {
	state := loadState()

	price24K, err := fetchGoldPricePerGram()
	if err != nil {
		log.Println("❌ Price fetch failed:", err)
		return
	}

	price22K := convertTo22K(price24K)

	log.Printf("💰 Gold 24K: ₹%.2f / g\n", price24K)
	log.Printf("💰 Gold 22K: ₹%.2f / g\n", price22K)

	// 🔔 ALERT
	if price22K <= targetGoldPrice22K && !state.AlertTriggered {
		msg := fmt.Sprintf(
			"22K Gold hit ₹%.2f / gram\nTarget: ₹%.2f",
			price22K,
			targetGoldPrice22K,
		)

		if err := sendPushover(msg); err != nil {
			log.Println("❌ Pushover error:", err)
			return
		}

		log.Println("✅ Pushover alert sent")
		state.AlertTriggered = true
		saveState(state)
	}

	// 🔄 RESET LOGIC
	if price22K > targetGoldPrice22K+resetBuffer && state.AlertTriggered {
		log.Println("🔄 Resetting alert state")
		state.AlertTriggered = false
		saveState(state)
	}
}

func fetchGoldPricePerGram() (float64, error) {
	req, _ := http.NewRequest(
		"GET",
		"https://www.goldapi.io/api/XAU/INR",
		nil,
	)
	req.Header.Set("x-access-token", goldAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("goldapi status %d", resp.StatusCode)
	}

	var data GoldAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, err
	}

	const ounceToGram = 31.1035
	return data.PricePerOunce / ounceToGram, nil
}

func sendPushover(message string) error {
	form := url.Values{}
	form.Add("token", pushoverAppToken)
	form.Add("user", pushoverUserKey)
	form.Add("title", "🚨 Gold Price Alert")
	form.Add("message", message)
	form.Add("priority", "1")

	resp, err := http.PostForm(
		"https://api.pushover.net/1/messages.json",
		form,
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("pushover returned %d", resp.StatusCode)
	}

	return nil
}

func convertTo22K(price24K float64) float64 {
	const purityFactor = 0.916
	const gstFactor = 1.03
	return price24K * purityFactor * gstFactor
}

func loadState() State {
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return State{}
	}
	var s State
	_ = json.Unmarshal(data, &s)
	return s
}

func saveState(s State) {
	data, _ := json.MarshalIndent(s, "", "  ")
	_ = os.WriteFile(stateFile, data, 0644)
}
