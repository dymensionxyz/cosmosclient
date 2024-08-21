package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"

	"github.com/dymensionxyz/cosmosclient/cosmosclient"
)

func main() {
	var eventQuery string

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

	// Subscribe to an event (examples)
	eventQuery = "tm.event = 'NewBlock'"
	// eventQuery = "tm.event = 'Tx'"
	// eventQuery = "tm.event='Tx' AND message.action='/cosmos.bank.v1beta1.Msg/Send'"
	// eventQuery = "tm.event='Tx' AND message.module='bank'"

	subscription, err := rpcClient.WSEvents.Subscribe(context.Background(), fmt.Sprintf("example-client-%d", rand.Int()), eventQuery)
	if err != nil {
		log.Fatalf("Failed to subscribe to event: %v", err)
	}
	fmt.Println("Subscribed to event:", eventQuery)

	// Listen for events
	for msg := range subscription {
		fmt.Println("Received event:", msg)
	}
}
