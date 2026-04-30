package vault

import (
	"net/http"
	"os"
)

func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        secret := os.Getenv("VAULT_SECRET_KEY")

        clientKey := r.Header.Get("X-Vault-API-Key")

        if clientKey == "" || clientKey != secret {
            http.Error(w, "Unauthorized: Invalid API Key", http.StatusUnauthorized)
            return
        }

        next(w, r)
    }
}