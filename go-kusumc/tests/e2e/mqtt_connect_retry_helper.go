package e2e

import (
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func connectMQTTWithRetry(opts *mqtt.ClientOptions, timeout time.Duration) (mqtt.Client, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error

	for {
		client := mqtt.NewClient(opts)
		tok := client.Connect()
		if tok.WaitTimeout(8*time.Second) && tok.Error() == nil {
			return client, nil
		}
		lastErr = tok.Error()
		if client.IsConnected() {
			client.Disconnect(200)
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("connect timeout")
	}
	return nil, fmt.Errorf("mqtt connect failed after %s: %w", timeout, lastErr)
}
