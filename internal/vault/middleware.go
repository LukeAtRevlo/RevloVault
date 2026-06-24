package vault

import (
	"crypto/subtle"
	"log"
	"net/http"
	"os"
)

func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        secret := os.Getenv("VAULT_SECRET_KEY")

        clientKey := r.Header.Get("X-Vault-API-Key")
        log.Printf("Auth: clientKey=%q secretSet=%v", clientKey, secret != "")

        if clientKey == "" || subtle.ConstantTimeCompare([]byte(clientKey), []byte(secret)) != 1 {
            http.Error(w, "Unauthorized: Invalid API Key", http.StatusUnauthorized)
            return
        }

        next(w, r)
    }
}