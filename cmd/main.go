package main

import (
	"log"
	"os"

	"github.com/GoogleCloudPlatform/functions-framework-go/funcframework"
	_ "github.com/torbendury/pubsub-shovel"
)

func main() {
	// Register the function handler for local testing
	//http.HandleFunc("/", shovel.Handler)
	//http.HandleFunc("/ShovelMessages", shovel.Handler)

	// Start the server
	port := "8080"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	log.Printf("Starting server on port %s", port)
	log.Printf("Function available at:")
	log.Printf("  http://localhost:%s/Handler", port)

	if err := funcframework.Start(port); err != nil {
		log.Fatalf("funcframework.Start: %v\n", err)
	}

	//if err := http.ListenAndServe(":"+port, nil); err != nil {
	//	log.Fatalf("Failed to start server: %v", err)
	//}
}
