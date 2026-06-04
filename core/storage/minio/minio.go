// Package minio implements storage.Provider for self-hosted MinIO.
// MinIO is fully S3-compatible and preferred for on-premise practices with
// strict NZ data sovereignty requirements (data never leaves the building).
// Endpoint format: http://localhost:9000 or https://minio.practiceinternal.nz
package minio

import (
	"context"

	"github.com/spf13/viper"

	"github.com/PhillipC05/tpt-healthcare/core/storage"
	"github.com/PhillipC05/tpt-healthcare/core/storage/s3"
)

func init() {
	storage.Register("minio", func(ctx context.Context, v *viper.Viper) (storage.Provider, error) {
		return s3.New(ctx, s3.Config{
			AccessKeyID:     v.GetString("storage.minio.access_key_id"),
			SecretAccessKey: v.GetString("storage.minio.secret_access_key"),
			Region:          "us-east-1", // MinIO ignores region but SigV4 requires one
			Bucket:          v.GetString("storage.minio.bucket"),
			BaseEndpoint:    v.GetString("storage.minio.endpoint"), // e.g. http://localhost:9000
		})
	})
}
