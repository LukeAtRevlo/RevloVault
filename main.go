package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
	"github.com/LukeAtRevlo/RevloVault/internal/vault"
	"google.golang.org/api/option" // Required for the credentials file
)

func main() {
	ctx := context.Background()

	// The path where Render mounts your Secret File
	const credsPath = "/etc/secrets/gcp-key.json"

	// 1. Validate Environment Variables
	bucketName := os.Getenv("GCS_BUCKET_NAME")
	if bucketName == "" {
		log.Fatal("GCS_BUCKET_NAME environment variable is required")
	}

	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		log.Fatal("GOOGLE_CLOUD_PROJECT environment variable is required for Firestore")
	}

	// 2. Initialize GCS Client
	// We use option.WithCredentialsFile to point to the Render Secret File
	gcsClient, err := storage.NewClient(ctx, option.WithCredentialsFile(credsPath))
	if err != nil {
		log.Fatalf("Failed to create GCS client: %v", err)
	}
	defer gcsClient.Close()

	// 3. Initialize Firestore Client
	firestoreClient, err := firestore.NewClient(ctx, projectID, option.WithCredentialsFile(credsPath))
	if err != nil {
		log.Fatalf("Failed to create Firestore client: %v", err)
	}
	defer firestoreClient.Close()

	// 4. Initialize Services and Handlers
	vaultService := vault.NewVaultService(gcsClient, firestoreClient, bucketName)
	vaultHandler := &vault.VaultHandler{Service: vaultService}

	// 5. Route Registration
	http.HandleFunc("/grant-upload", vault.AuthMiddleware(vaultHandler.HandleGrantUpload))
	http.HandleFunc("/grant-download", vault.AuthMiddleware(vaultHandler.HandleGrantDownload))
	
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// 6. Server Start
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("RevloVault listening on :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}