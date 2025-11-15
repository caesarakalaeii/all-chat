package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/caesar/all-chat/shared/auth"
)

func main() {
	service := flag.String("service", "", "Service name the token should represent")
	secret := flag.String("secret", "", "Signing secret used by SERVICE_JWT_SECRET")
	expiry := flag.Duration("expiry", 24*time.Hour, "How long the token should remain valid")
	flag.Parse()

	if *service == "" {
		log.Fatal("service flag is required")
	}

	if *secret == "" {
		log.Fatal("secret flag is required")
	}

	token, err := auth.GenerateServiceJWT(*service, *secret, *expiry)
	if err != nil {
		log.Fatalf("failed to generate token: %v", err)
	}

	fmt.Println(token)
}
