// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package internal

import (
	"reflect"
	"testing"

	"github.com/lancedb/lancedb-go/pkg/contracts"
)

func TestLegacyStorageOptionsKeepV012S3Scope(t *testing.T) {
	access, secret, token, region, profile := "access", "secret", "token", "us-east-1", "primary"
	endpoint, serverSide, kms, class := "http://127.0.0.1:1", "AES256", "kms-key", "STANDARD"
	forcePath, anonymous, useSSL := true, true, false
	options := &contracts.StorageOptions{S3Config: &contracts.S3Config{
		AccessKeyID:       &access,
		SecretAccessKey:   &secret,
		SessionToken:      &token,
		Region:            &region,
		Endpoint:          &endpoint,
		ForcePathStyle:    &forcePath,
		Profile:           &profile,
		AnonymousAccess:   &anonymous,
		UseSSL:            &useSSL,
		ServerSideEncrypt: &serverSide,
		SSEKMSKeyID:       &kms,
		StorageClass:      &class,
	}}

	got := legacyStorageOptions(options)
	want := map[string]string{
		contracts.StorageAWSAccessKeyID:     access,
		contracts.StorageAWSSecretAccessKey: secret,
		contracts.StorageAWSSessionToken:    token,
		contracts.StorageAWSRegion:          region,
		contracts.StorageAWSProfile:         profile,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy storage options = %#v, want only the v0.1.2 S3 fields %#v", got, want)
	}
	for _, key := range []string{contracts.StorageAWSEndpoint, contracts.StorageVirtualHostedStyleRequest, "skip_signature", contracts.StorageAllowHTTP} {
		if _, exists := got[key]; exists {
			t.Errorf("legacy storage options activated %q; use ConnectWithStorageOptions for explicit endpoint/path/anonymous/TLS switches", key)
		}
	}
}
