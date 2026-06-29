package vault

import (
	"crypto/subtle"
	"log"
	"net/http"
	"os"
	"strings"
)

func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        secret := strings.TrimSpace(os.Getenv("VAULT_SECRET_KEY"))

        clientKey := r.Header.Get("X-Vault-API-Key")
        log.Printf("Auth: clientKeyLen=%d secretLen=%d", len(clientKey), len(secret))

        if clientKey == "" || subtle.ConstantTimeCompare([]byte(clientKey), []byte(secret)) != 1 {

            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        next(w, r)
    }
}