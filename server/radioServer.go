package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os/exec"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RadioStation represents a radio station with its name and stream URL.
type RadioStation struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

var (
	mpvCmd     *exec.Cmd
	mpvMutex   sync.Mutex
	stations   []RadioStation
	currentIdx int
	currentVol int = 50 // Default volume
)

// FetchRadioStations fetches the list of radio stations from an external API.
// func FetchRadioStations(apiURL string) ([]RadioStation, error) {
// 	resp, err := http.Get(apiURL)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer resp.Body.Close()

// 	body, err := ioutil.ReadAll(resp.Body)
// 	if err != nil {
// 		return nil, err
// 	}

// 	var fetchedStations []RadioStation
// 	if err := json.Unmarshal(body, &fetchedStations); err != nil {
// 		return nil, err
// 	}

// 	return fetchedStations, nil
// }

func FetchRadioStations(apiURL string) ([]RadioStation, error) {
	// Create a custom HTTP client with timeout settings
	client := &http.Client{
		Timeout: 20 * time.Second, // Set a timeout for the request
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // Only use this if you need to skip TLS verification (not recommended for production)
			},
		},
	}

	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var fetchedStations []RadioStation
	if err := json.Unmarshal(body, &fetchedStations); err != nil {
		return nil, err
	}

	return fetchedStations, nil
}

// StartMPV starts the mpv process with the given stream URL.
func StartMPV(url string) error {
	mpvMutex.Lock()
	defer mpvMutex.Unlock()

	// Kill existing mpv process if it's running
	if mpvCmd != nil && mpvCmd.Process != nil {
		log.Println("Stopping existing mpv process...")
		mpvCmd.Process.Kill()
	}

	// Start a new mpv process
	log.Printf("Starting mpv with URL: %s\n", url)
	mpvCmd = exec.Command("mpv", "--no-video", "--idle=yes", "--input-ipc-server=/tmp/mpv-socket", url)
	if err := mpvCmd.Start(); err != nil {
		return fmt.Errorf("failed to start mpv: %v", err)
	}

	return nil
}

// SendMPVCommand sends a command to the running mpv process via IPC.
func SendMPVCommand(command string) error {
	log.Printf("Sending command to mpv: %s\n", command)
	// Create the command properly without using echo and pipe
	cmd := exec.Command("socat", "-", "/tmp/mpv-socket")

	// Create a pipe to write to socat's stdin
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %v", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start socat: %v", err)
	}

	// Write the command to stdin
	_, err = fmt.Fprintln(stdin, command)
	if err != nil {
		return fmt.Errorf("failed to write to stdin: %v", err)
	}

	// Close stdin to signal we're done writing
	stdin.Close()

	// Wait for the command to complete
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("socat command failed: %v", err)
	}

	return nil
}

// PlayNextStation switches to the next station in the list.
func PlayNextStation() error {
	currentIdx = (currentIdx + 1) % len(stations)
	log.Printf("Playing next station: %s\n", stations[currentIdx].Name)
	return StartMPV(stations[currentIdx].URL)
}

func PlayPrevStation() error {
	currentIdx = (currentIdx - 1 + len(stations)) % len(stations)
	log.Printf("Playing prev station: %s\n", stations[currentIdx].Name)
	return StartMPV(stations[currentIdx].URL)
}

// AdjustVolume changes the volume by a given delta.
// func AdjustVolume(delta int) error {
// 	currentVol += delta
// 	if currentVol < 0 {
// 		currentVol = 0
// 	} else if currentVol > 100 {
// 		currentVol = 100
// 	}

// 	log.Printf("Adjusting volume to: %d\n", currentVol)
// 	command := fmt.Sprintf(`{ "command": ["set_property", "volume", %d] }`, currentVol)
// 	return SendMPVCommand(command)
// }

func AdjustVolume(delta int) error {
	currentVol += delta
	if currentVol < 0 {
		currentVol = 0
	} else if currentVol > 100 {
		currentVol = 100
	}

	log.Printf("Adjusting volume to: %d\n", currentVol)
	// Format the command properly as a JSON string
	command := fmt.Sprintf(`{"command": ["set_property", "volume", %d]}`, currentVol)
	return SendMPVCommand(command)
}

func main() {
	r := gin.Default()

	// Fetch stations on startup
	var err error
	apiURL := "https://xmedia.workers.dev" // Replace with your external API URL
	stations, err = FetchRadioStations(apiURL)
	if err != nil {
		log.Fatalf("Failed to fetch radio stations: %v", err)
	}

	// Endpoint to fetch and list radio stations
	r.GET("/stations", func(c *gin.Context) {
		c.JSON(http.StatusOK, stations)
	})

	// Endpoint to play a specific radio stream
	r.POST("/play", func(c *gin.Context) {
		if err := StartMPV(stations[0].URL); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Playing one")})
	})

	// Endpoint to play the next station
	r.POST("/next", func(c *gin.Context) {
		if err := PlayNextStation(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to play next station: %v", err)})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Playing next station: %s", stations[currentIdx].Name)})
	})

	r.POST("/prev", func(c *gin.Context) {
		if err := PlayPrevStation(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to play prev station: %v", err)})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Playing next station: %s", stations[currentIdx].Name)})
	})

	// Endpoint to increase volume
	r.POST("/volup", func(c *gin.Context) {
		if err := AdjustVolume(10); err != nil { // Increase volume by 10
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to adjust volume: %v", err)})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Volume increased to %d", currentVol)})
	})

	// Endpoint to decrease volume
	r.POST("/voldown", func(c *gin.Context) {
		if err := AdjustVolume(-10); err != nil { // Decrease volume by 10
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to adjust volume: %v", err)})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Volume decreased to %d", currentVol)})
	})

	// Run the server
	log.Println("Server started on :8080")
	r.Run(":8080")
}
