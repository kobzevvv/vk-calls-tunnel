package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/kobzevvv/vk-calls-tunnel/internal/turn"
)

func main() {
	vkLink := "https://vk.com/call/join/rghzHz3-z775Ls6UScecjC-tOoJp3qLwVZ00hZm5vjw"

	fmt.Println("Fetching TURN credentials from VK call link...")
	fmt.Printf("Link: %s\n\n", vkLink)

	creds, err := turn.FetchFromLink(vkLink)
	if err != nil {
		log.Fatalf("FetchFromLink failed: %v", err)
	}

	fmt.Println("=== TURN Credentials ===")
	fmt.Printf("Username:     %s\n", creds.Username)
	fmt.Printf("Password:     %s\n", creds.Password)
	fmt.Printf("TURN Servers: %s\n", strings.Join(creds.TURNServers, ", "))
	fmt.Printf("Server count: %d\n", len(creds.TURNServers))
}
