#!/usr/bin/env python3
"""
Simple Twitch chat bot to send test messages for debugging.
Uses the caesarlp bot account to send messages to caesarlp's channel.
"""

import socket
import time
import sys
import os
from pathlib import Path

# Load credentials from the nearest .env file (searching up from this script,
# so it finds the repo-root .env regardless of where this file lives).
def load_env():
    """Load environment variables from the nearest .env file up the tree."""
    for parent in Path(__file__).resolve().parents:
        env_path = parent / ".env"
        if env_path.exists():
            with open(env_path) as f:
                for line in f:
                    line = line.strip()
                    if line and not line.startswith("#") and "=" in line:
                        key, value = line.split("=", 1)
                        os.environ[key] = value
            return

load_env()

# Twitch IRC configuration
TWITCH_HOST = "irc.chat.twitch.tv"
TWITCH_PORT = 6667
TWITCH_NICK = os.getenv("TWITCH_BOT_USERNAME")
TWITCH_TOKEN = os.getenv("TWITCH_BOT_OAUTH")

if not TWITCH_NICK or not TWITCH_TOKEN:
    print("[ERROR] Missing required environment variables!")
    print("Please ensure .env file exists with:")
    print("  TWITCH_BOT_USERNAME=your_bot_username")
    print("  TWITCH_BOT_OAUTH=oauth:your_token")
    sys.exit(1)

TWITCH_CHANNEL = f"#{TWITCH_NICK}"

def send_message(sock, message):
    """Send a message to the Twitch channel."""
    sock.send(f"PRIVMSG {TWITCH_CHANNEL} :{message}\r\n".encode("utf-8"))
    print(f"[SENT] {message}")

def connect_to_twitch():
    """Connect to Twitch IRC and join the channel."""
    sock = socket.socket()
    sock.connect((TWITCH_HOST, TWITCH_PORT))

    # Authenticate
    sock.send(f"PASS {TWITCH_TOKEN}\r\n".encode("utf-8"))
    sock.send(f"NICK {TWITCH_NICK}\r\n".encode("utf-8"))
    sock.send(f"JOIN {TWITCH_CHANNEL}\r\n".encode("utf-8"))

    print(f"[INFO] Connected to Twitch IRC as {TWITCH_NICK}")
    print(f"[INFO] Joined channel {TWITCH_CHANNEL}")

    # Read initial IRC messages
    time.sleep(2)
    try:
        sock.settimeout(1.0)
        response = sock.recv(2048).decode("utf-8")
        print(f"[IRC] {response}")
    except socket.timeout:
        pass
    sock.settimeout(None)

    return sock

def main():
    """Main function to send test messages."""
    print("=" * 60)
    print("Twitch Chat Test Bot - caesarlp")
    print("=" * 60)

    sock = connect_to_twitch()

    try:
        # Send test messages with different content
        messages = [
            "Test message #1 - Basic message",
            "Test message #2 - With emoji 😀",
            "Test message #3 - 7TV emotes KEKW WHERE",
            "Test message #4 - More emotes KEKW KEKW WHERE",
            "Test message #5 - Mixed Kappa KEKW WHERE LUL",
        ]

        for i, msg in enumerate(messages, 1):
            print(f"\n[{i}/{len(messages)}] Sending message...")
            send_message(sock, msg)
            time.sleep(2)  # Wait 2 seconds between messages

            # Handle PING/PONG to keep connection alive
            sock.settimeout(0.1)
            try:
                response = sock.recv(2048).decode("utf-8")
                if response.startswith("PING"):
                    sock.send("PONG :tmi.twitch.tv\r\n".encode("utf-8"))
                    print("[IRC] Responded to PING")
            except socket.timeout:
                pass
            sock.settimeout(None)

        print("\n" + "=" * 60)
        print("All test messages sent successfully!")
        print("=" * 60)

    except KeyboardInterrupt:
        print("\n[INFO] Interrupted by user")
    except Exception as e:
        print(f"[ERROR] {e}")
        sys.exit(1)
    finally:
        sock.close()
        print("[INFO] Disconnected from Twitch IRC")

if __name__ == "__main__":
    main()
