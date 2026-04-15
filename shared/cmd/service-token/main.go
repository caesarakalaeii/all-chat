// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

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
