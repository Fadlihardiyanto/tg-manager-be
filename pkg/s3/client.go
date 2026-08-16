package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"go.uber.org/zap"
)

// Client wraps the AWS S3 client for S3-compatible storage (R2, MinIO, AWS S3).
type Client struct {
	s3Client  *s3.Client
	bucket    string
	publicURL string
	endpoint  string
	logger    *zap.Logger
}

// Config holds S3-compatible storage settings (decoupled from internal/config to avoid import cycles).
type Config struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	BucketName      string
	Region          string
	UsePathStyle    bool   // true for MinIO, false for R2/AWS
	PublicURL       string // custom public CDN/domain for serving files
}

// UploadInput holds parameters for uploading a file.
type UploadInput struct {
	Key         string
	Body        io.Reader
	ContentType string
	Size        int64 // optional, -1 if unknown
}

// UploadOutput holds the result of an upload operation.
type UploadOutput struct {
	Key       string
	PublicURL string
	ETag      string
}

// PresignInput holds parameters for generating a presigned URL.
type PresignInput struct {
	Key       string
	ExpiresIn time.Duration
}

// NewClient creates a new S3-compatible client from the provided config.
func NewClient(cfg *Config, logger *zap.Logger) (*Client, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("s3: endpoint is required")
	}
	if cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" {
		return nil, fmt.Errorf("s3: access key and secret key are required")
	}

	// Build custom resolver for S3-compatible endpoints (R2, MinIO)
	customResolver := aws.EndpointResolverWithOptionsFunc(
		func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:               cfg.Endpoint,
				HostnameImmutable: true,
			}, nil
		},
	)

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		),
		awsconfig.WithEndpointResolverWithOptions(customResolver),
	)
	if err != nil {
		return nil, fmt.Errorf("s3: failed to load config: %w", err)
	}

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.UsePathStyle
	})

	// Verify connection by checking if bucket exists
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = s3Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(cfg.BucketName),
	})
	if err != nil {
		logger.Warn("s3: bucket check failed (may not exist or no permission)",
			zap.String("bucket", cfg.BucketName),
			zap.Error(err),
		)
	} else {
		logger.Info("s3: successfully connected to storage",
			zap.String("endpoint", cfg.Endpoint),
			zap.String("bucket", cfg.BucketName),
			zap.String("region", cfg.Region),
		)
	}

	return &Client{
		s3Client:  s3Client,
		bucket:    cfg.BucketName,
		publicURL: cfg.PublicURL,
		endpoint:  cfg.Endpoint,
		logger:    logger,
	}, nil
}

// Upload uploads a file to S3 and returns the result with public URL.
func (c *Client) Upload(ctx context.Context, input *UploadInput) (*UploadOutput, error) {
	if input == nil {
		return nil, fmt.Errorf("s3: upload input is nil")
	}
	putInput := &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(input.Key),
		Body:        input.Body,
		ContentType: aws.String(input.ContentType),
	}

	if input.Size > 0 {
		putInput.ContentLength = aws.Int64(input.Size)
	}

	resp, err := c.s3Client.PutObject(ctx, putInput)
	if err != nil {
		return nil, fmt.Errorf("s3: upload failed: %w", err)
	}

	etag := ""
	if resp.ETag != nil {
		etag = *resp.ETag
	}

	return &UploadOutput{
		Key:       input.Key,
		PublicURL: c.GetPublicURL(input.Key),
		ETag:      etag,
	}, nil
}

// Delete removes a file from S3.
func (c *Client) Delete(ctx context.Context, key string) error {
	_, err := c.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("s3: delete failed for key %s: %w", key, err)
	}
	return nil
}

// DeleteBatch removes multiple files from S3 with automatic chunking.
// S3 DeleteObjects API has a limit of 1000 objects per request,
// so this function splits the keys into chunks of 1000 and processes them sequentially.
func (c *Client) DeleteBatch(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	const batchSize = 1000 // S3 API limit per DeleteObjects request

	for start := 0; start < len(keys); start += batchSize {
		end := start + batchSize
		if end > len(keys) {
			end = len(keys)
		}

		chunk := keys[start:end]
		objects := make([]s3types.ObjectIdentifier, len(chunk))
		for i, key := range chunk {
			objects[i] = s3types.ObjectIdentifier{
				Key: aws.String(key),
			}
		}

		resp, err := c.s3Client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(c.bucket),
			Delete: &s3types.Delete{
				Objects: objects,
				Quiet:   aws.Bool(false), // return errors for individual failures
			},
		})
		if err != nil {
			return fmt.Errorf("s3: batch delete failed (chunk %d-%d): %w", start, end-1, err)
		}

		// Report individual object-level errors — partial delete bukan silent
		// success: caller (mis. purge data user) harus tahu objek tersisa.
		perr := false
		for _, e := range resp.Errors {
			perr = true
			key := ""
			if e.Key != nil {
				key = *e.Key
			}
			code := ""
			if e.Code != nil {
				code = *e.Code
			}
			message := ""
			if e.Message != nil {
				message = *e.Message
			}
			c.logger.Warn("s3: individual object delete failed in batch",
				zap.String("key", key),
				zap.String("code", code),
				zap.String("message", message),
			)
		}
		if perr {
			return fmt.Errorf("s3: batch delete partial failure in chunk %d-%d", start, end-1)
		}
	}

	return nil
}

// PresignGet generates a presigned GET URL for temporary access to a private file.
func (c *Client) PresignGet(ctx context.Context, input *PresignInput) (string, error) {
	if input == nil {
		return "", fmt.Errorf("s3: presign input is nil")
	}
	presignClient := s3.NewPresignClient(c.s3Client)

	resp, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(input.Key),
	}, s3.WithPresignExpires(input.ExpiresIn))
	if err != nil {
		return "", fmt.Errorf("s3: presign failed for key %s: %w", input.Key, err)
	}

	return resp.URL, nil
}

// PresignPut generates a presigned PUT URL for direct client uploads.
func (c *Client) PresignPut(ctx context.Context, input *PresignInput, contentType string) (string, error) {
	if input == nil {
		return "", fmt.Errorf("s3: presign input is nil")
	}
	presignClient := s3.NewPresignClient(c.s3Client)

	resp, err := presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(input.Key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(input.ExpiresIn))
	if err != nil {
		return "", fmt.Errorf("s3: presign put failed for key %s: %w", input.Key, err)
	}

	return resp.URL, nil
}

// Exists checks if an object exists in the bucket.
// Returns (true, nil) if the object exists, (false, nil) if it does not,
// or (false, err) if an infrastructure/permission error occurred.
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	_, err := c.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		// Properly distinguish 404 (object not found) from infrastructure errors.
		// S3 HeadObject returns a NotFound error type for missing objects.
		var notFound *s3types.NotFound
		if errors.As(err, &notFound) {
			return false, nil
		}
		return false, fmt.Errorf("s3: exists check failed for key %s: %w", key, err)
	}
	return true, nil
}

// GetPublicURL returns the public URL for a given object key.
// If PublicURL is configured (CDN), it uses that as the base.
// Otherwise, it constructs the URL from the endpoint.
func (c *Client) GetPublicURL(key string) string {
	if c.publicURL != "" {
		return fmt.Sprintf("%s/%s", c.publicURL, key)
	}
	// Fallback: construct from endpoint (works for MinIO with public bucket)
	return fmt.Sprintf("%s/%s/%s", c.endpoint, c.bucket, key)
}

// Bucket returns the configured bucket name.
func (c *Client) Bucket() string {
	return c.bucket
}
