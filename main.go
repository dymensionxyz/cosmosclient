package main

import (
	"context"
	"fmt"
	"log"

	"github.com/dymensionxyz/cosmosclient/cosmosclient"
)

func main() {
	// Create a new RPC client

	options := []cosmosclient.Option{
		cosmosclient.WithAddressPrefix("dym"),
		cosmosclient.WithNodeAddress("http://localhost:36657"),
	}

	rpcClient, err := cosmosclient.New(options...)
	if err != nil {
		log.Fatalf("Failed to create RPC client: %v", err)
	}
	err = rpcClient.RPC.Start()
	if err != nil {
		log.Fatalf("Failed to start RPC client: %v", err)
	}

	// Subscribe to an event
	eventQuery := "tm.event = 'NewBlock'"
	subscription, err := rpcClient.WSEvents.Subscribe(context.Background(), "example-client", eventQuery)
	if err != nil {
		log.Fatalf("Failed to subscribe to event: %v", err)
	}
	fmt.Println("Subscribed to event:", eventQuery)

	// Listen for events
	for {
		select {
		case msg := <-subscription:
			fmt.Println("Received event:", msg)
		}
	}
}
