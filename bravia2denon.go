package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"
)

// Replace with your device IPs or hostnames
const (
	sonyAPIURL = "http://192.168.20.20/sony/"
	denonIP    = "192.168.20.18:23" // Replace with actual Denon control URL
	authPSK    = "1234"             // Your Sony PSK
)

// Function to check Sony TV power status

/* Example API call:

	curl -H "Content-Type: application/json" -H "X-Auth-PSK: 1234" -X POST -d \
    '{"id": 20, "method": "getPowerStatus", "id": 55, "params": [{"status": false}], "version": "1.0"}'  http://192.168.20.20/sony/system
	{"result":[{"status":"active"}],"id":55}

*/

func getSonyPowerStatus() (bool, error) {

	requestBody := map[string]interface{}{
		"method":  "getPowerStatus",
		"id":      50,
		"params":  []map[string]bool{},
		"version": "1.0",
	}
	respBody, err := doPost(sonyAPIURL+"system", requestBody)
	if err != nil {
		return false, err
	}

	var respMap map[string]interface{}
	if err := json.Unmarshal(respBody, &respMap); err != nil {
		return false, err
	}

	resultArr, ok := respMap["result"].([]interface{})
	if !ok || len(resultArr) == 0 {
		return false, fmt.Errorf("invalid response: %v", respMap)
	}

	result, ok := resultArr[0].(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("invalid result format")
	}

	status, ok := result["status"].(string)
	if !ok {
		return false, fmt.Errorf("status not found")
	}

	if status == "active" {
		// TV is ON
		return true, nil
	} else {
		// TV is OFF
		return false, nil
	}
}

// Function to get TV volume

/* API example:

   curl -H "Content-Type: application/json" -H "X-Auth-PSK: 1234" -X POST -d \
   '{"method": "getVolumeInformation", "id": 33, "params": [], "version": "1.0"}'  http://192.168.20.20/sony/audio
   {"result":[[{"target":"speaker","volume":3,"mute":false,"maxVolume":100,"minVolume":0}]],"id":33}

*/

func getTVVolume() (int, error) {

	requestBody := map[string]interface{}{
		"method":  "getVolumeInformation",
		"id":      33,
		"params":  []map[string]bool{},
		"version": "1.0",
	}
	respBody, err := doPost(sonyAPIURL+"audio", requestBody)
	if err != nil {
		return 0, err
	}

	var respMap map[string]interface{}
	if err := json.Unmarshal(respBody, &respMap); err != nil {
		return 0, err
	}

	resultArr, ok := respMap["result"].([]interface{})
	if !ok || len(resultArr) == 0 {
		return 0, fmt.Errorf("invalid response: %v", respMap)
	}

	volumeInfoArr, ok := resultArr[0].([]interface{})
	if !ok || len(volumeInfoArr) == 0 {
		return 0, fmt.Errorf("invalid volume info format")
	}

	// Get volume from the first item
	volumeInfo, ok := volumeInfoArr[0].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("invalid volume info structure")
	}

	volumeFloat, ok := volumeInfo["volume"].(float64)
	if !ok {
		return 0, fmt.Errorf("volume not found or invalid type")
	}

	volume := int(volumeFloat)

	// Get mute status
	mute, ok := volumeInfo["mute"].(bool)
	if !ok {
		return 0, fmt.Errorf("mute status not found or invalid type")
	}
	if mute {
		volume = 0 // If muted, treat volume as 0
	}

	return volume, nil
}

// DENON AVR control protocol
// https://assets.denon.com/documentmaster/uk/avr1713_avr1613_protocol_v860.pdf

func sendDenonCommand(command string) (string, error) {
	conn, err := net.DialTimeout("tcp", denonIP, 1*time.Second)
	if err != nil {
		return "", fmt.Errorf("connection error: %w", err)
	}
	defer conn.Close()

	//time.Sleep(100 * time.Millisecond)
	// Send the command
	_, err = conn.Write([]byte(command + "\r\n"))
	if err != nil {
		return "", fmt.Errorf("write error: %w", err)
	}

	if !strings.Contains(command, "?") {
		// If it's not a query command, no need to read response
		return "", nil
	}

	// Small delay to allow processing
	time.Sleep(100 * time.Millisecond)

	// Read the response
	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\r')
	if err != nil {
		return "", fmt.Errorf("read error: %w", err)
	}

	response = strings.TrimSpace(response)
	return response, nil
}

// Helper function to perform POST requests
func doPost(url string, body interface{}) ([]byte, error) {
	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-PSK", authPSK)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return ioutil.ReadAll(resp.Body)
}

// Main loop
func main() {

	for {

		// Small delay to avoid overwhelming the devices
		time.Sleep(1000 * time.Millisecond)

		// Check if TV is ON
		fmt.Println("Checking TV power status...")
		powerStatus, err := getSonyPowerStatus()
		if err != nil {
			fmt.Println("Error checking TV power:", err)
			time.Sleep(10 * time.Second)
			continue
		}

		if !powerStatus {
			fmt.Println("TV is not ON.")
			time.Sleep(10 * time.Second)
			continue
		}

		fmt.Println("TV is ON, checking volume...")
		volume, err := getTVVolume()
		if err != nil {
			fmt.Println("Error getting TV volume:", err)
			time.Sleep(10 * time.Second)
			continue
		}
		fmt.Printf("Bravia TV volume is %d\n", volume)

		fmt.Println("Checking Denon status...")
		denonStatus, err := sendDenonCommand("PW?")
		if err != nil {
			fmt.Println("Error checking Denon status:", err)
			return
		}
		if denonStatus != "PWON" {
			fmt.Println("Denon is OFF.")
			time.Sleep(1 * time.Second)
			continue
		}

		fmt.Println("Denon is ON, checking volume...")
		denonVolumeResp, err := sendDenonCommand("MV?")
		if err != nil {
			fmt.Println("Error getting Denon volume:", err)
			time.Sleep(1 * time.Second)
			continue
		}
		denonVolumeResp = strings.TrimPrefix(denonVolumeResp, "MV")
		denonVolume := 0
		fmt.Sscanf(denonVolumeResp, "%d", &denonVolume)
		fmt.Printf("Denon volume is %d\n", denonVolume)

		// Limit volume to max 40
		if volume > 40 {
			volume = 40
		}

		// Adjust Denon volume if different
		if denonVolume != volume {
			fmt.Printf("--> Setting Denon volume to %d\n", volume)
			_, err := sendDenonCommand(fmt.Sprintf("MV%02d", volume))
			if err != nil {
				fmt.Println("Error setting Denon volume:", err)
				time.Sleep(1 * time.Second)
				continue
			}
		}
	} // Endless loop
}
