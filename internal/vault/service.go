package vault

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

type VaultService struct {
	s3Client      *s3.Client
	presignClient *s3.PresignClient
	bucketName    string
	encryptionKey []byte
}

func NewVaultService(s3Client *s3.Client, bucketName string, hexKey string) (*VaultService, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil || len(key) != 32 {
		return nil, errors.New("vault init error: master key must be a valid 32-byte hex string")
	}

	return &VaultService{
		s3Client:      s3Client,
		presignClient: s3.NewPresignClient(s3Client),
		bucketName:    bucketName,
		encryptionKey: key,
	}, nil
}

// encrypt string using standard AES-GCM
func (s *VaultService) encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return "vault:v1:" + hex.EncodeToString(ciphertext), nil
}

// decrypt string using standard AES-GCM
func (s *VaultService) decrypt(ciphertext string) (string, error) {
	if !strings.HasPrefix(ciphertext, "vault:v1:") {
		return "", errors.New("invalid or unencrypted ciphertext format")
	}
	
	pureHex := strings.TrimPrefix(ciphertext, "vault:v1:")
	data, err := hex.DecodeString(pureHex)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, actualCiphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, actualCiphertext, nil)
	if err != nil {
		return "", errors.New("decryption failed: data may be corrupted or key is incorrect")
	}

	return string(plaintext), nil
}

func (s *VaultService) GetUploadURL(ctx context.Context, fileName string, orgId string, clientId string, contentType string) (string, string, error) {
	id := uuid.New().String()
	
	// 1. This is the structural raw storage path for your S3 bucket
	rawKey := fmt.Sprintf("vault/%s/%s/%d_%s_%s", orgId, clientId, time.Now().Unix(), id, fileName)

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// 2. Request the presigned upload URL from S3 using the REAL path
	req, err := s.presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucketName),
		Key:         aws.String(rawKey),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(15*time.Minute))
	if err != nil {
		return "", "", err
	}

	// 3. Encrypt the raw path into an unreadable string right before returning it
	encryptedKey, err := s.encrypt(rawKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to blind storage path: %w", err)
	}

	// Return the upload URL and the SECURE encrypted key string to save to your Main Database
	return req.URL, encryptedKey, nil
}

func (s *VaultService) GetDownloadURL(ctx context.Context, encryptedKey string, orgId string) (string, error) {
	rawKey, err := s.decrypt(encryptedKey)
	if err != nil {
		return "", err
	}

	if !strings.HasPrefix(rawKey, "vault/"+orgId+"/") {
		return "", errors.New("forbidden: key does not belong to this org")
	}

	// 2. Pass the real path to S3 to build the 10-minute temporary link
	req, err := s.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(rawKey),
	}, s3.WithPresignExpires(10*time.Minute))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

func (s *VaultService) VerifyAccess(ctx context.Context, encryptedKey string) error {
	rawKey, err := s.decrypt(encryptedKey)
	if err != nil {
		return fmt.Errorf("key decryption failed: %w", err)
	}

	_, err = s.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(rawKey),
	})
	if err != nil {
		return fmt.Errorf("file not found in storage: %w", err)
	}
	return nil
}

func (s *VaultService) ReadFile(ctx context.Context, encryptedKey string) ([]byte, error) {
	// 1. Decrypt the blinded database string back into the actual S3 path string
	rawKey, err := s.decrypt(encryptedKey)
	if err != nil {
		return nil, err
	}

	result, err := s.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(rawKey),
	})
	if err != nil {
		return nil, err
	}
	defer result.Body.Close()
	return io.ReadAll(result.Body)
}