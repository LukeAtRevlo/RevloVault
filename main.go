package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/LukeAtRevlo/RevloVault/internal/audit"
	"github.com/LukeAtRevlo/RevloVault/internal/check"
	"github.com/LukeAtRevlo/RevloVault/internal/vault"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	ctx := context.Background()

	bucketName := os.Getenv("S3_BUCKET_NAME")
	if bucketName == "" {
		log.Fatal("S3_BUCKET_NAME environment variable is required")
	}

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			os.Getenv("S3_ACCESS_KEY_ID"),
			os.Getenv("S3_SECRET_KEY"),
			"",
		)),
	)
	if err != nil {
		log.Fatalf("Failed to load S3 config: %v", err)
	}

	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(os.Getenv("S3_ENDPOINT"))
		o.UsePathStyle = true
	})

	vaultService, err := vault.NewVaultService(s3Client, bucketName, os.Getenv("VAULT_MASTER_KEY"))
	if err != nil {
		log.Fatalf("Failed to init vault: %v", err)
	}

	valueDB, err := pgxpool.New(ctx, os.Getenv("VAULT_DB_URL"))
	if err != nil {
		log.Fatalf("Failed to connect to vault values database: %v", err)
	}
	defer valueDB.Close()

	vaultHandler := &vault.VaultHandler{
		Service: vaultService,
		Values:  vault.NewValueService(valueDB),
	}



	checkService := check.NewCheckService(os.Getenv("SUMSUB_TOKEN"), os.Getenv("SUMSUB_SECRET"))
	amlService := check.NewAMLService(os.Getenv("OPENSANCTIONS_API_KEY"))
	checkHandler := &check.CheckHandler{
		Service:       checkService,
		AMLService:    amlService,
		VaultService:  vaultService,
		WebhookSecret: os.Getenv("SUMSUB_WEBHOOK_SECRET"),
	}


	db, err := pgx.Connect(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("Failed to connect to postgres: %v", err)
	}
	defer db.Close(ctx)

	auditService := audit.NewAuditService(db)
	auditHandler := audit.NewAuditHandler(auditService)

	http.HandleFunc("/grant-upload", vault.AuthMiddleware(vaultHandler.HandleGrantUpload))
	http.HandleFunc("/grant-download", vault.AuthMiddleware(vaultHandler.HandleGrantDownload))
	http.HandleFunc("/vault/verify", vault.AuthMiddleware(vaultHandler.HandleVerifyAccess))
	http.HandleFunc("/store-value", vault.AuthMiddleware(vaultHandler.HandleStoreValue))
	http.HandleFunc("/get-value", vault.AuthMiddleware(vaultHandler.HandleGetValue))
	http.HandleFunc("/verification/aml", vault.AuthMiddleware(checkHandler.HandleAML))
	http.HandleFunc("/verification/identity", vault.AuthMiddleware(checkHandler.HandleIdentity))
	http.HandleFunc("/verification/webhook", checkHandler.HandleWebhook)
	http.HandleFunc("/vault/audit", vault.AuthMiddleware(auditHandler.HandleCreateAudit))
	http.HandleFunc("/vault/audits", vault.AuthMiddleware(auditHandler.HandleGetAudits))


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
