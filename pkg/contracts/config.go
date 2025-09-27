package contracts

// StorageOptions holds storage-related options
type StorageOptions struct {
	// AWS S3 Options
	S3Config *S3Config `json:"s3_config,omitempty"`

	// Azure Blob Storage Options
	AzureConfig *AzureConfig `json:"azure_config,omitempty"`

	// Google Cloud Storage Options
	GCSConfig *GCSConfig `json:"gcs_config,omitempty"`

	// Local File System Options
	LocalConfig *LocalConfig `json:"local_config,omitempty"`

	// General Options
	BlockSize          *int    `json:"block_size,omitempty"`             // Block size in bytes
	MaxRetries         *int    `json:"max_retries,omitempty"`            // Maximum retry attempts
	RetryDelay         *int    `json:"retry_delay,omitempty"`            // Retry delay in milliseconds
	Timeout            *int    `json:"timeout,omitempty"`                // Request timeout in seconds
	AllowHTTP          *bool   `json:"allow_http,omitempty"`             // Allow HTTP connections (insecure)
	UserAgent          *string `json:"user_agent,omitempty"`             // Custom User-Agent header
	ConnectTimeout     *int    `json:"connect_timeout,omitempty"`        // Connection timeout in seconds
	ReadTimeout        *int    `json:"read_timeout,omitempty"`           // Read timeout in seconds
	PoolIdleTimeout    *int    `json:"pool_idle_timeout,omitempty"`      // Connection pool idle timeout
	PoolMaxIdlePerHost *int    `json:"pool_max_idle_per_host,omitempty"` // Max idle connections per host
}

// S3Config holds AWS S3 specific configuration
type S3Config struct {
	AccessKeyID       *string `json:"access_key_id,omitempty"`
	SecretAccessKey   *string `json:"secret_access_key,omitempty"`
	SessionToken      *string `json:"session_token,omitempty"`
	Region            *string `json:"region,omitempty"`
	Endpoint          *string `json:"endpoint,omitempty"`            // Custom endpoint (e.g., MinIO)
	ForcePathStyle    *bool   `json:"force_path_style,omitempty"`    // Use path-style addressing
	Profile           *string `json:"profile,omitempty"`             // AWS profile name
	AnonymousAccess   *bool   `json:"anonymous_access,omitempty"`    // Use anonymous access
	UseSSL            *bool   `json:"use_ssl,omitempty"`             // Use HTTPS
	ServerSideEncrypt *string `json:"server_side_encrypt,omitempty"` // Server-side encryption
	SSEKMSKeyID       *string `json:"sse_kms_key_id,omitempty"`      // KMS key ID for encryption
	StorageClass      *string `json:"storage_class,omitempty"`       // Storage class (STANDARD, IA, etc.)
}

// AzureConfig holds Azure Blob Storage specific configuration
type AzureConfig struct {
	AccountName  *string `json:"account_name,omitempty"`
	AccessKey    *string `json:"access_key,omitempty"`
	SasToken     *string `json:"sas_token,omitempty"`
	TenantID     *string `json:"tenant_id,omitempty"`
	ClientID     *string `json:"client_id,omitempty"`
	ClientSecret *string `json:"client_secret,omitempty"`
	Authority    *string `json:"authority,omitempty"`      // Authority URL
	Endpoint     *string `json:"endpoint,omitempty"`       // Custom endpoint
	UseHTTPS     *bool   `json:"use_https,omitempty"`      // Use HTTPS
	UseManagedID *bool   `json:"use_managed_id,omitempty"` // Use managed identity
}

// GCSConfig holds Google Cloud Storage specific configuration
type GCSConfig struct {
	ServiceAccountPath     *string `json:"service_account_path,omitempty"` // Path to service account JSON
	ServiceAccountKey      *string `json:"service_account_key,omitempty"`  // Service account JSON as string
	ProjectID              *string `json:"project_id,omitempty"`
	ApplicationCredentials *string `json:"application_credentials,omitempty"` // GOOGLE_APPLICATION_CREDENTIALS
	Endpoint               *string `json:"endpoint,omitempty"`                // Custom endpoint
	AnonymousAccess        *bool   `json:"anonymous_access,omitempty"`        // Use anonymous access
	UseSSL                 *bool   `json:"use_ssl,omitempty"`                 // Use HTTPS
}

// LocalConfig holds local file system specific configuration
type LocalConfig struct {
	CreateDirIfNotExists *bool `json:"create_dir_if_not_exists,omitempty"` // Create directory if it doesn't exist
	UseMemoryMap         *bool `json:"use_memory_map,omitempty"`           // Use memory mapping for files
	SyncWrites           *bool `json:"sync_writes,omitempty"`              // Sync writes to disk immediately
}
