package main

import (
	"log"

	"github.com/gokuls-codes/go-gen-ui/internal/ai"
	"github.com/gokuls-codes/go-gen-ui/internal/expenses"
	"github.com/gokuls-codes/go-gen-ui/internal/jev"
	"github.com/gokuls-codes/go-gen-ui/internal/server"
	"github.com/joho/godotenv"
)

func main() {

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	aiClient, err := ai.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	jevClient, err := jev.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	expenseStore, err := expenses.NewStore("data/expenses.json")
	if err != nil {
		log.Fatal(err)
	}

	server := server.NewServer(":8080", aiClient, jevClient, expenseStore)
	err = server.Start()
	if err != nil {
		log.Fatal(err)
	}
}
