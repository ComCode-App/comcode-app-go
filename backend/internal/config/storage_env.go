package config

// StorageFromEnv reads storage settings from the environment without loading any .env
// file or requiring unrelated server authentication configuration (for offline tools).
func StorageFromEnv() StorageConfig { return loadStorageConfig() }
