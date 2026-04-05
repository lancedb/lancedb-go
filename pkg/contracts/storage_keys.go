// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package contracts

// --- S3 / S3-Compatible (MinIO, R2) ---

const (
	// StorageAccessKeyID is the AWS access key ID.
	StorageAccessKeyID = "access_key_id"

	// StorageSecretAccessKey is the AWS secret access key.
	StorageSecretAccessKey = "secret_access_key"

	// StorageSessionToken is the AWS session token for temporary credentials.
	StorageSessionToken = "session_token"

	// StorageRegion is the AWS region (e.g., "us-east-1").
	StorageRegion = "region"

	// StorageEndpoint is the custom S3 endpoint URL.
	// For R2, use StorageAWSEndpoint instead to enable constant-size upload parts.
	StorageEndpoint = "endpoint"

	// StorageAWSEndpoint is the AWS endpoint URL.
	// Required for R2 (must use this key, not StorageEndpoint) to trigger
	// Lance's constant-size upload part detection.
	// Format for R2: https://<ACCOUNT_ID>.r2.cloudflarestorage.com
	StorageAWSEndpoint = "aws_endpoint"

	// StorageVirtualHostedStyleRequest controls whether to use virtual-hosted-style
	// requests (true) or path-style requests (false). Set to "false" for MinIO.
	StorageVirtualHostedStyleRequest = "virtual_hosted_style_request"

	// StorageUnsignedPayload enables unsigned payloads for S3 requests.
	StorageUnsignedPayload = "unsigned_payload"

	// StorageConditionalPut configures conditional put behavior.
	StorageConditionalPut = "conditional_put"

	// StorageCopyIfNotExists configures copy-if-not-exists behavior.
	StorageCopyIfNotExists = "copy_if_not_exists"

	// StorageS3Express enables S3 Express One Zone support.
	StorageS3Express = "s3_express"

	// StorageRoleArn is the ARN of an IAM role to assume.
	StorageRoleArn = "role_arn"

	// StorageRoleSessionName is the session name for assumed role sessions.
	StorageRoleSessionName = "role_session_name"

	// StorageWebIdentityTokenFile is the path to a web identity token file.
	StorageWebIdentityTokenFile = "web_identity_token_file"

	// StorageDefaultRegion is the fallback region when region is not set.
	StorageDefaultRegion = "default_region"

	// StorageBucket is the S3 bucket name (usually inferred from the URI).
	StorageBucket = "bucket"

	// StorageSkipSignature skips request signing (for public buckets).
	StorageSkipSignature = "skip_signature"

	// StorageDisableTagging disables object tagging.
	StorageDisableTagging = "disable_tagging"

	// StorageRequestPayer sets the request payer (e.g., "requester").
	StorageRequestPayer = "request_payer"
)

// --- Google Cloud Storage ---

const (
	// StorageGCSServiceAccount is the path to a GCS service account JSON file.
	// Also accepts the alias "google_service_account_path".
	StorageGCSServiceAccount = "google_service_account"

	// StorageGCSServiceAccountKey is the GCS service account JSON key as a string.
	StorageGCSServiceAccountKey = "google_service_account_key"

	// StorageGCSApplicationCredentials is the path set by GOOGLE_APPLICATION_CREDENTIALS.
	StorageGCSApplicationCredentials = "google_application_credentials"

	// StorageGCSBucket is the GCS bucket name.
	StorageGCSBucket = "google_bucket"
)

// --- Azure Blob Storage ---

const (
	// StorageAzureAccountName is the Azure storage account name.
	StorageAzureAccountName = "azure_storage_account_name"

	// StorageAzureAccessKey is the Azure storage access key.
	StorageAzureAccessKey = "azure_storage_account_key"

	// StorageAzureSASToken is the Azure Shared Access Signature token.
	StorageAzureSASToken = "azure_storage_sas_token"

	// StorageAzureTenantID is the Azure Active Directory tenant ID.
	StorageAzureTenantID = "azure_storage_tenant_id"

	// StorageAzureClientID is the Azure Active Directory client ID.
	StorageAzureClientID = "azure_storage_client_id"

	// StorageAzureClientSecret is the Azure Active Directory client secret.
	StorageAzureClientSecret = "azure_storage_client_secret"

	// StorageAzureAuthorityID is the Azure authority URL.
	StorageAzureAuthorityID = "azure_storage_authority_id"

	// StorageAzureContainerName is the Azure blob container name.
	StorageAzureContainerName = "azure_container_name"

	// StorageAzureEndpoint is the Azure storage endpoint URL.
	StorageAzureEndpoint = "azure_storage_endpoint"

	// StorageAzureUseFabricEndpoint configures use of Azure Fabric endpoint.
	StorageAzureUseFabricEndpoint = "azure_use_fabric_endpoint"

	// StorageAzureMSIEndpoint is the endpoint for Azure Managed Service Identity.
	StorageAzureMSIEndpoint = "azure_msi_endpoint"

	// StorageAzureUseAzureCLI enables authentication via Azure CLI.
	StorageAzureUseAzureCLI = "azure_use_azure_cli"
)

// --- General / Client Options ---

const (
	// StorageAllowHTTP allows HTTP connections (not HTTPS). Required for
	// non-TLS endpoints like local MinIO. Value: "true" or "false".
	StorageAllowHTTP = "allow_http"

	// StorageAllowInvalidCertificates allows invalid TLS certificates.
	// Value: "true" or "false".
	StorageAllowInvalidCertificates = "allow_invalid_certificates"

	// StorageConnectTimeout is the connection timeout. Value is a duration
	// string (e.g., "5s", "30s").
	StorageConnectTimeout = "connect_timeout"

	// StorageTimeout is the request timeout. Value is a duration string.
	StorageTimeout = "timeout"

	// StorageUserAgent sets a custom User-Agent header.
	StorageUserAgent = "user_agent"

	// StorageProxyURL sets a proxy URL for all requests.
	StorageProxyURL = "proxy_url"
)
