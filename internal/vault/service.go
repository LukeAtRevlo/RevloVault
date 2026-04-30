package vault

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/storage"
	"github.com/google/uuid"
)

type VaultService struct {
	client *storage.Client
	bucketName string 
} 	

func NewVaultService(client *storage.Client, bucketName string) *VaultService{
	return &VaultService{client: client, bucketName: bucketName}
}	

func (s *VaultService) GetUploadURL(ctx context.Context, fileName string, orgId string, clientId string) (string, string, error) {
	id := uuid.New().String()
	key := fmt.Sprintf("uploads/%s/%s/%d_%s_%s", orgId, clientId, time.Now().Unix(), id, fileName)
	opts := &storage.SignedURLOptions{
        Scheme:  storage.SigningSchemeV4,
        Method:  "PUT",
        Expires: time.Now().Add(10 * time.Minute),
    }

	url, err := s.client.Bucket(s.bucketName).SignedURL(key, opts)

	return url, key, err
}

func (s *VaultService) GetDownloadURL(ctx context.Context, key string) (string, error){
	
	opts := &storage.SignedURLOptions{
		Scheme:  storage.SigningSchemeV4,
		Method:  "GET",
		Expires: time.Now().Add(10 * time.Minute),
	}

	url, err := s.client.Bucket(s.bucketName).SignedURL(key, opts)

	return url, err
}