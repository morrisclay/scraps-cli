package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Credential represents stored credentials for a host.
type Credential struct {
	APIKey   string `json:"api_key"`
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}

// Credentials is a map of host -> credential.
type Credentials map[string]Credential

// credentialsPath returns the path to the credentials file.
func credentialsPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "credentials.json"), nil
}

// LoadCredentials loads credentials from disk.
func LoadCredentials() (Credentials, error) {
	path, err := credentialsPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return make(Credentials), nil
	}
	if err != nil {
		return nil, err
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}

	return creds, nil
}

// SaveCredentials saves credentials to disk.
func SaveCredentials(creds Credentials) error {
	if err := ensureConfigDir(); err != nil {
		return err
	}

	path, err := credentialsPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// GetCredential returns the credential for a host.
// If host is empty, uses the default host.
func GetCredential(host string) (*Credential, error) {
	if host == "" {
		host = GetHost()
	}

	creds, err := LoadCredentials()
	if err != nil {
		return nil, err
	}

	cred, ok := creds[host]
	if !ok {
		return nil, nil
	}

	return &cred, nil
}

// SetCredential saves a credential for a host.
func SetCredential(host string, cred Credential) error {
	if host == "" {
		host = GetHost()
	}

	creds, err := LoadCredentials()
	if err != nil {
		creds = make(Credentials)
	}

	creds[host] = cred
	return SaveCredentials(creds)
}

// RemoveCredential removes the credential for a host.
func RemoveCredential(host string) error {
	if host == "" {
		host = GetHost()
	}

	creds, err := LoadCredentials()
	if err != nil {
		return err
	}

	delete(creds, host)
	return SaveCredentials(creds)
}

// HasCredential checks if there is a credential for the host.
func HasCredential(host string) bool {
	cred, err := GetCredential(host)
	return err == nil && cred != nil && cred.APIKey != ""
}
