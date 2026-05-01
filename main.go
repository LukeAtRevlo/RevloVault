package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/storage"
	"github.com/LukeAtRevlo/RevloVault/internal/vault"
)

func main() {
	ctx := context.Background()

	bucketName := os.Getenv("GCS_BUCKET_NAME")
	if bucketName == "" {
		log.Fatal("GCS_BUCKET_NAME environment variable is required")
	}

	gcsClient, err := storage.NewClient(ctx)

	if err != nil {
		log.Fatalf("Failed to create GCS client: %v", err)
	}
	defer gcsClient.Close()

	vaultService := vault.NewVaultService(gcsClient, bucketName)
	vaultHandler := &vault.VaultHandler{Service: vaultService}

	http.HandleFunc("/grant-upload", vault.AuthMiddleware(vaultHandler.HandleGrantUpload))
	http.HandleFunc("/grant-download", vault.AuthMiddleware(vaultHandler.HandleGrantDownload))
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("RevloVault listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))

}
