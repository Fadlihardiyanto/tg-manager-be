package usecase

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	pkg_s3 "github.com/Fadlihardiyanto/telegram-management-app/pkg/s3"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type IUploadUseCase interface {
	GeneratePresignedURL(ctx context.Context, clientID uuid.UUID, req *model.UploadPresignRequest) (*model.UploadPresignResponse, error)
}

type UploadUseCase struct {
	s3Client *pkg_s3.Client
	log      *zap.Logger
}

func NewUploadUseCase(s3Client *pkg_s3.Client, log *zap.Logger) IUploadUseCase {
	return &UploadUseCase{
		s3Client: s3Client,
		log:      log,
	}
}

func (uc *UploadUseCase) GeneratePresignedURL(ctx context.Context, clientID uuid.UUID, req *model.UploadPresignRequest) (*model.UploadPresignResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("upload usecase generate presigned url start", zap.String("client_id", clientID.String()), zap.String("file_name", req.FileName))

	if uc.s3Client == nil {
		log.Error("s3 client is not configured")
		return nil, fmt.Errorf("Penyimpanan S3 tidak dikonfigurasi")
	}

	// Generate a unique file name to avoid collisions
	ext := filepath.Ext(req.FileName)
	randomPrefix := uuid.New().String()[:8]
	uniqueFileName := fmt.Sprintf("%s_%d%s", randomPrefix, time.Now().Unix(), ext)

	// key structure: tenant_uploads/{client_id}/{unique_file_name}
	key := fmt.Sprintf("tenant_uploads/%s/%s", clientID.String(), uniqueFileName)

	// Generate presigned PUT URL (TTL 15 minutes)
	uploadURL, err := uc.s3Client.PresignPut(ctx, &pkg_s3.PresignInput{
		Key:       key,
		ExpiresIn: 15 * time.Minute,
	}, req.ContentType)
	if err != nil {
		log.Error("failed to generate presigned PUT URL", zap.Error(err))
		return nil, fmt.Errorf("Gagal membuat URL unggah: %w", err)
	}

	publicURL := uc.s3Client.GetPublicURL(key)

	log.Info("successfully generated presigned URL", zap.String("key", key))

	return &model.UploadPresignResponse{
		UploadURL: uploadURL,
		PublicURL: publicURL,
		Key:       key,
	}, nil
}
