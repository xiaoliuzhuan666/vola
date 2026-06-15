package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/agi-bar/vola/internal/vault"
	"github.com/jackc/pgx/v5/pgxpool"
)

type probeResult struct {
	Scope          string `json:"scope"`
	PlaintextBytes int    `json:"plaintext_bytes"`
}

var uuidPattern = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

func redactScope(scope string) string {
	return uuidPattern.ReplaceAllString(scope, "<uuid>")
}

func main() {
	databaseURL := flag.String("database-url", "", "Postgres database URL")
	masterKey := flag.String("vault-master-key", "", "hex VAULT_MASTER_KEY; defaults to VAULT_MASTER_KEY env")
	scope := flag.String("scope", "", "optional scope to probe")
	flag.Parse()

	if strings.TrimSpace(*databaseURL) == "" {
		fmt.Fprintln(os.Stderr, "missing --database-url")
		os.Exit(2)
	}
	masterKeyValue := strings.TrimSpace(*masterKey)
	if masterKeyValue == "" {
		masterKeyValue = strings.TrimSpace(os.Getenv("VAULT_MASTER_KEY"))
	}
	if masterKeyValue == "" {
		fmt.Fprintln(os.Stderr, "missing --vault-master-key")
		os.Exit(2)
	}

	v, err := vault.NewVault(masterKeyValue)
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid vault master key")
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, *databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect database: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	args := []any{}
	where := "octet_length(nonce) = 12 AND octet_length(encrypted_data) > 0"
	if strings.TrimSpace(*scope) != "" {
		args = append(args, strings.TrimSpace(*scope))
		where += " AND scope = $1"
	}

	rows, err := pool.Query(ctx, `
		SELECT scope, encrypted_data, nonce
		FROM vault_entries
		WHERE `+where+`
		ORDER BY updated_at DESC, created_at DESC
		LIMIT 20`, args...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "query vault entries: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	var failures []string
	for rows.Next() {
		var scopeValue string
		var ciphertext []byte
		var nonce []byte
		if err := rows.Scan(&scopeValue, &ciphertext, &nonce); err != nil {
			fmt.Fprintf(os.Stderr, "scan vault entry: %v\n", err)
			os.Exit(1)
		}
		plaintext, err := v.Decrypt(ciphertext, nonce)
		if err != nil {
			failures = append(failures, redactScope(scopeValue))
			continue
		}
		result := probeResult{
			Scope:          redactScope(scopeValue),
			PlaintextBytes: len(plaintext),
		}
		encoded, _ := json.Marshal(result)
		fmt.Println(string(encoded))
		return
	}
	if err := rows.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "read vault entries: %v\n", err)
		os.Exit(1)
	}
	if len(failures) > 0 {
		fmt.Fprintf(os.Stderr, "vault decrypt failed for %d candidate(s); first_scope=%s\n", len(failures), failures[0])
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "no encrypted vault entry candidates found")
	os.Exit(1)
}
