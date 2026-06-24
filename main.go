package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
	"github.com/LukeAtRevlo/RevloVault/internal/check"
	"github.com/LukeAtRevlo/RevloVault/internal/vault"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	ctx := context.Background()

	// If the static service account JSON text is found, write it to disk
	if keyContent := os.Getenv("GCP_KEY_CONTENT"); keyContent != "" {
		err := os.WriteFile("gcp-key.json", []byte(keyContent), 0644)
		if err != nil {
			log.Fatalf("Failed to write static service account key file: %v", err)
		}
		os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "gcp-key.json")
	}

	bucketName := os.Getenv("GCS_BUCKET_NAME")
	if bucketName == "" {
		log.Fatal("GCS_BUCKET_NAME environment variable is required")
	}

	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		log.Fatal("GOOGLE_CLOUD_PROJECT environment variable is required for Firestore")
	}

	// Clients automatically look at GOOGLE_APPLICATION_CREDENTIALS path
	gcsClient, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create GCS client: %v", err)
	}
	defer gcsClient.Close()

	firestoreClient, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("Failed to create Firestore client: %v", err)
	}
	defer firestoreClient.Close()

	vaultService := vault.NewVaultService(gcsClient, firestoreClient, bucketName, os.Getenv("GCP_SERVICE_ACCOUNT"))
	vaultHandler := &vault.VaultHandler{Service: vaultService}

	checkService := check.NewCheckService(os.Getenv("SUMSUB_TOKEN"), os.Getenv("SUMSUB_SECRET"))
	checkHandler := &check.CheckHandler{Service: checkService, VaultService: vaultService}

	http.HandleFunc("/grant-upload", vault.AuthMiddleware(vaultHandler.HandleGrantUpload))
	http.HandleFunc("/grant-download", vault.AuthMiddleware(vaultHandler.HandleGrantDownload))
	http.HandleFunc("/check/applicant", vault.AuthMiddleware(checkHandler.HandleCreateApplicant))
	http.HandleFunc("/check/aml", vault.AuthMiddleware(checkHandler.HandleRecheckAML))
	http.HandleFunc("/check/document", vault.AuthMiddleware(checkHandler.HandleSubmitDocument))
	http.HandleFunc("/check/submit", vault.AuthMiddleware(checkHandler.HandleSubmit))
	
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("RevloVault listening on :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}