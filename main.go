package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"

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
	eventQuery := ""
	eventQuery = "state_update.rollapp_id='rollappevm_1234-1'"
	eventQuery = "tm.event='Tx' AND message.action='/cosmos.bank.v1beta1.Msg/Send'"
	eventQuery = "tm.event='Tx' AND message.module='bank'"
	eventQuery = "tm.event = 'Tx'"
	eventQuery = "tm.event = 'Tx' AND coin_received.receiver = 'dym17xpfvakm2amg962yls6f84z3kell8c5lzy0xwn'"
	eventQuery = "coin_received.receiver = 'dym1ssx7j96d9cxestj55f05z93e36cd6nmj2rz5zv'"
	eventQuery = "tm.event = 'NewBlock'"

	subscription, err := rpcClient.WSEvents.Subscribe(context.Background(), fmt.Sprintf("example-client-%d", rand.Int()), eventQuery)
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
