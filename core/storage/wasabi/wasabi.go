// Package wasabi implements storage.Provider for Wasabi Cloud Storage.
// Wasabi is fully S3-compatible with lower egress costs. AP region (ap-southeast-1)
// is the closest to NZ; use the Sydney region if Wasabi opens one.
// Endpoint: s3.ap-southeast-1.wasabisys.com
// No egress fees make Wasabi cost-effective for large radiology/DICOM files.
package wasabi

import (
	"context"
	"fmt"

	"github.com/spf13/viper"

	"github.com/PhillipC05/tpt-healthcare/core/storage"
	"github.com/PhillipC05/tpt-healthcare/core/storage/s3"
)

func init() {
	storage.Register("wasabi", func(ctx context.Context, v *viper.Viper) (storage.Provider, error) {
		region := v.GetString("storage.wasabi.region")
		if region == "" {
			region = "ap-southeast-1"
		}
		return s3.New(ctx, s3.Config{
			AccessKeyID:     v.GetString("storage.wasabi.access_key_id"),
			SecretAccessKey: v.GetString("storage.wasabi.secret_access_key"),
			Region:          region,
			Bucket:          v.GetString("storage.wasabi.bucket"),
			// Wasabi uses regional S3-compatible endpoints.
			BaseEndpoint: fmt.Sprintf("https://s3.%s.wasabisys.com", region),
		})
	})
}
