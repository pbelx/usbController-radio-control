import struct
import time
import os
from subprocess import Popen

class GamepadShortcuts:
    def __init__(self, device_path="/dev/input/js0"):
        self.device_path = device_path
        self.button_mappings = {
            0: 'http POST lo:8080/play',  # Example command
            1: 'http POST lo:8080/next',                    # Example command
            2: 'http POST lo:8080/prev',                     # Example command
            3: 'killall mpv',
	    6: 'http POST lo:8080/voldown',
            7: 'http POST lo:8080/volup'
            # Add more button mappings as needed
        }
        
        # Format for parsing joystick input
        # long timestamp, short value, unsigned char type, unsigned char number
        self.FORMAT = 'IhBB'
        self.EVENT_SIZE = struct.calcsize(self.FORMAT)
        
        # Store button states to prevent repeated triggering
        self.button_states = {button: False for button in self.button_mappings}

    def execute_command(self, command):
        """Execute the shell command"""
        try:
            Popen(command, shell=True)
        except Exception as e:
            print(f"Error executing command: {e}")

    def process_event(self, event):
        """Process gamepad events and trigger commands"""
        timestamp, value, event_type, button_number = event
        
        # Type 1 is button events
        if event_type == 1:
            # Check if the button is mapped
            if button_number in self.button_mappings:
                # Check if button state changed to pressed (value = 1)
                if value == 1 and not self.button_states[button_number]:
                    print(f"Executing command for button {button_number}")
                    self.execute_command(self.button_mappings[button_number])
                
                # Update button state
                self.button_states[button_number] = bool(value)

    def run(self):
        """Main loop to read gamepad events"""
        print("Gamepad shortcut mapper running. Press Ctrl+C to exit.")
        print("Current mappings:")
        for button, command in self.button_mappings.items():
            print(f"Button {button}: {command}")

        try:
            # Open the joystick device
            with open(self.device_path, "rb") as device:
                while True:
                    # Read one complete event
                    event = struct.unpack(self.FORMAT, device.read(self.EVENT_SIZE))
                    self.process_event(event)
                    
        except KeyboardInterrupt:
            print("\nExiting...")
        except (FileNotFoundError, PermissionError) as e:
            print(f"Error: {e}")
            print("Make sure the device exists and you have permission to access it.")
            print("Try running with sudo if you get a permission error.")

if __name__ == "__main__":
    shortcuts = GamepadShortcuts()
    shortcuts.run()
