package vault

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
	"github.com/google/uuid"
)

type VaultService struct {
	storageClient *storage.Client
	db            *firestore.Client
	bucketName    string
}

func NewVaultService(storageClient *storage.Client, db *firestore.Client, bucketName string) *VaultService {
	return &VaultService{
		storageClient: storageClient,
		db:            db,
		bucketName:    bucketName,
	}
}

func (s *VaultService) GetUploadURL(ctx context.Context, fileName string, orgId string, clientId string) (string, string, error) {
	id := uuid.New().String()
	key := fmt.Sprintf("vault/%s/%s/%d_%s_%s", orgId, clientId, time.Now().Unix(), id, fileName)
	
	opts := &storage.SignedURLOptions{
		Scheme:  storage.SigningSchemeV4,
		Method:  "PUT",
		Expires: time.Now().Add(15 * time.Minute),
		Headers: []string{"Content-Type:application/octet-stream"},
	}

	url, err := s.storageClient.Bucket(s.bucketName).SignedURL(key, opts)
	if err != nil {
		return "", "", err
	}

	return url, key, nil
}

func (s *VaultService) GetDownloadURL(ctx context.Context, key string) (string, error) {
	opts := &storage.SignedURLOptions{
		Scheme:  storage.SigningSchemeV4,
		Method:  "GET",
		Expires: time.Now().Add(10 * time.Minute),
	}

	url, err := s.storageClient.Bucket(s.bucketName).SignedURL(key, opts)
	return url, err
}
