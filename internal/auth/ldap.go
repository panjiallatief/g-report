package auth

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/go-ldap/ldap/v3"
)

// LDAPConfig holds LDAP connection settings loaded from environment
type LDAPConfig struct {
	Server string // LDAP server address (e.g., "172.20.70.7:389")
	Domain string // LDAP domain (e.g., "beritasatu.id")
	BaseDN string // Base DN for search (e.g., "dc=beritasatu,dc=id")
}

// GetLDAPConfig returns LDAP configuration from environment variables
func GetLDAPConfig() LDAPConfig {
	return LDAPConfig{
		Server: os.Getenv("LDAP_URL"),
		Domain: os.Getenv("LDAP_DOMAIN"),
		BaseDN: getEnvOrDefault("LDAP_BASE_DN", "dc=beritasatu,dc=id"),
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// AuthenticateLDAP authenticates a user against the LDAP server
// Returns fullName, department on success, or error on failure
func AuthenticateLDAP(username, password string) (fullName, department string, err error) {
	if username == "" {
		return "", "", fmt.Errorf("username cannot be empty")
	}
	if password == "" {
		return "", "", fmt.Errorf("password cannot be empty")
	}

	config := GetLDAPConfig()

	if config.Server == "" || config.Domain == "" {
		return "", "", fmt.Errorf("LDAP configuration is incomplete (LDAP_URL or LDAP_DOMAIN not set)")
	}

	// Normalize username to lowercase
	username = strings.ToLower(username)

	// Connect to the LDAP server
	l, err := ldap.Dial("tcp", config.Server)
	if err != nil {
		log.Printf("[LDAP] Failed to connect to server %s: %v", config.Server, err)
		return "", "", fmt.Errorf("failed to connect to LDAP server: %v", err)
	}
	defer l.Close()

	// Bind with the provided username and password
	// Format: username@domain (e.g., john.doe@beritasatu.id)
	bindDN := fmt.Sprintf("%s@%s", username, config.Domain)
	err = l.Bind(bindDN, password)
	if err != nil {
		log.Printf("[LDAP] Failed to bind user %s: %v", username, err)
		return "", "", fmt.Errorf("invalid credentials")
	}

	// Search for user to get additional attributes (cn, department)
	userFilter := fmt.Sprintf("(sAMAccountName=%s)", ldap.EscapeFilter(username))
	searchRequest := ldap.NewSearchRequest(
		config.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0, 0, false,
		userFilter,
		[]string{"dn", "cn", "department"},
		nil,
	)

	result, err := l.Search(searchRequest)
	if err != nil {
		log.Printf("[LDAP] Search failed for user %s: %v", username, err)
		return "", "", fmt.Errorf("failed to search for user: %v", err)
	}

	// Extract user attributes
	if len(result.Entries) > 0 {
		fullName = result.Entries[0].GetAttributeValue("cn")
		department = result.Entries[0].GetAttributeValue("department")
		log.Printf("[LDAP] User %s authenticated successfully (Name: %s, Department: %s)", username, fullName, department)
	} else {
		// User authenticated but not found in search - use username as display name
		fullName = username
		department = ""
		log.Printf("[LDAP] User %s authenticated but not found in directory search", username)
	}

	// Default department if not found
	if department == "" {
		department = "DEFAULT"
	}

	return fullName, department, nil
}

// IsLDAPEnabled checks if LDAP authentication is enabled
func IsLDAPEnabled() bool {
	return os.Getenv("LDAP_URL") != "" && os.Getenv("LDAP_DOMAIN") != ""
}

// IsDevelopmentMode checks if the application is running in development mode
func IsDevelopmentMode() bool {
	return os.Getenv("ENVIRONMENT") == "development"
}
