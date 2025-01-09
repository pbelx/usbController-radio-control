package main

import (
    "encoding/binary"
    "fmt"
    "io"
    "net/http"
    "os"
)

// JoystickEvent represents the structure of a joystick input event
type JoystickEvent struct {
    Time   uint32
    Value  int16
    Type   uint8
    Number uint8
}

// GamepadShortcuts handles gamepad input and command execution
type GamepadShortcuts struct {
    devicePath     string
    buttonMappings map[uint8]func() error
    buttonStates   map[uint8]bool
    client         *http.Client
}

// NewGamepadShortcuts creates a new GamepadShortcuts instance
func NewGamepadShortcuts(devicePath string) *GamepadShortcuts {
    if devicePath == "" {
        devicePath = "/dev/input/js0"
    }

    g := &GamepadShortcuts{
        devicePath:   devicePath,
        buttonStates: make(map[uint8]bool),
        client:      &http.Client{},
    }

    // Initialize button mappings with HTTP client
    g.buttonMappings = map[uint8]func() error{
        0: g.postRequest("http://localhost:8080/play"),
        1: g.postRequest("http://localhost:8080/next"),
        2: g.postRequest("http://localhost:8080/prev"),
        3: g.postRequest("http://localhost:8080/stop"), // Changed to use HTTP endpoint for stop
        6: g.postRequest("http://localhost:8080/voldown"),
        7: g.postRequest("http://localhost:8080/volup"),
    }

    return g
}

// postRequest creates a function that performs an HTTP POST request to the given URL
func (g *GamepadShortcuts) postRequest(url string) func() error {
    return func() error {
        req, err := http.NewRequest(http.MethodPost, url, nil)
        if err != nil {
            return fmt.Errorf("error creating request: %v", err)
        }
        
        resp, err := g.client.Do(req)
        if err != nil {
            return fmt.Errorf("error making request: %v", err)
        }
        defer resp.Body.Close()

        // Read and print the response body
        body, err := io.ReadAll(resp.Body)
        if err != nil {
            return fmt.Errorf("error reading response: %v", err)
        }
        
        fmt.Printf("Response from %s:\nStatus: %s\nBody: %s\n", 
            url, 
            resp.Status,
            string(body),
        )

        if resp.StatusCode != http.StatusOK {
            return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
        }
        
        return nil
    }
}

// processEvent handles gamepad events and triggers commands
func (g *GamepadShortcuts) processEvent(event JoystickEvent) {
    // Type 1 is button events
    if event.Type == 1 {
        // Check if the button is mapped
        if handler, exists := g.buttonMappings[event.Number]; exists {
            // Check if button state changed to pressed (value = 1)
            if event.Value == 1 && !g.buttonStates[event.Number] {
                fmt.Printf("Executing command for button %d\n", event.Number)
                if err := handler(); err != nil {
                    fmt.Printf("Error executing command: %v\n", err)
                }
            }
            // Update button state
            g.buttonStates[event.Number] = event.Value == 1
        }
    }
}

// Run starts the main event loop
func (g *GamepadShortcuts) Run() error {
    fmt.Println("Gamepad shortcut mapper running. Press Ctrl+C to exit.")
    fmt.Println("Current mappings:")
    for button := range g.buttonMappings {
        var action string
        switch button {
        case 0:
            action = "Play"
        case 1:
            action = "Next"
        case 2:
            action = "Previous"
        case 3:
            action = "Stop"
        case 6:
            action = "Volume Down"
        case 7:
            action = "Volume Up"
        }
        fmt.Printf("Button %d: %s\n", button, action)
    }

    // Open the joystick device
    device, err := os.OpenFile(g.devicePath, os.O_RDONLY, 0)
    if err != nil {
        return fmt.Errorf("error opening device: %v\nMake sure the device exists and you have permission to access it", err)
    }
    defer device.Close()

    event := JoystickEvent{}
    for {
        err := binary.Read(device, binary.LittleEndian, &event)
        if err != nil {
            return fmt.Errorf("error reading event: %v", err)
        }
        g.processEvent(event)
    }
}

func main() {
    shortcuts := NewGamepadShortcuts("")
    if err := shortcuts.Run(); err != nil {
        fmt.Printf("Error: %v\n", err)
        os.Exit(1)
    }
}
