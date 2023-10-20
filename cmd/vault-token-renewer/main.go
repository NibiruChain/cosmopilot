package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	k8s "github.com/postfinance/vaultk8s"
)

func main() {
	c, err := k8s.NewFromEnvironment()
	if err != nil {
		log.Fatal(err)
	}

	// get token and re-authenticate if enabled
	token, err := c.GetToken()
	if err != nil {
		log.Fatal(err)
	}

	renewer, err := c.NewRenewer(token)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("start renewer loop")

	go renewer.Renew()

	exit := make(chan os.Signal, 1)
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case err := <-renewer.DoneCh():
			if err != nil {
				log.Fatal(fmt.Errorf("failed to renew token: %w", err))
			}

			os.Exit(0)
		case <-renewer.RenewCh():
			log.Println("token renewed")
		case <-exit:
			log.Println("signal received - stop execution")
			renewer.Stop()
			return
		}
	}
}
