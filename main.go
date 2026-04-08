package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"bug-bounty-engine/ai"
	"bug-bounty-engine/api"
	"bug-bounty-engine/engine"
	"bug-bounty-engine/server"
)

func main() {
	client := &http.Client{Timeout: 20 * time.Second}
	fetchService := api.NewDefaultServiceFromEnv(client)
	core := engine.New(fetchService)
	explainer := ai.NewGroqMultiAgentFromEnv(client)

	httpServer := server.New(core, "Frontend", explainer)
	port := stringsOrDefault(os.Getenv("PORT"), "8080")
	addr := ":" + port

	log.Printf("starting bug bounty priority engine at %s", addr)
	if err := http.ListenAndServe(addr, httpServer.Handler()); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func stringsOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
