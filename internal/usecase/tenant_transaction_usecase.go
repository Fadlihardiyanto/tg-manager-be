package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model/converter"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	pkg_s3 "github.com/Fadlihardiyanto/telegram-management-app/pkg/s3"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// receiptPresignExpiry is the TTL for presigned receipt download URLs.
const receiptPresignExpiry = 15 * time.Minute

// ITenantTransactionUseCase defines the contract for tenant-scoped transaction (order history) operations.
type ITenantTransactionUseCase interface {
	// FindAll returns a paginated list of transactions (orders) for the tenant.
	FindAll(ctx context.Context, clientID uuid.UUID, filter model.TransactionFilterRequest) ([]model.TransactionResponse, int64, error)
}

type tenantTransactionUseCase struct {
	db        *entity.Database
	orderRepo repository.IOrderRepository
	s3Client  *pkg_s3.Client
	log       *zap.Logger
}

func NewTenantTransactionUseCase(
	db *entity.Database,
	orderRepo repository.IOrderRepository,
	s3Client *pkg_s3.Client,
	log *zap.Logger,
) ITenantTransactionUseCase {
	return &tenantTransactionUseCase{
		db:        db,
		orderRepo: orderRepo,
		s3Client:  s3Client,
		log:       log,
	}
}

// FindAll fetches paginated transactions with full JOIN data (package, user, discount).
// For orders that have a receipt, a time-limited presigned S3 URL is generated.
func (uc *tenantTransactionUseCase) FindAll(ctx context.Context, clientID uuid.UUID, filter model.TransactionFilterRequest) ([]model.TransactionResponse, int64, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("tenant transaction usecase FindAll start", zap.String("client_id", clientID.String()))

	// 1. Fetch paginated orders with preloaded Package, User, Discount
	orders, err := uc.orderRepo.FindTransactionsByClientID(ctx, uc.db.Gorm, clientID, filter)
	if err != nil {
		log.Error("tenant transaction usecase FindAll fetch failed", zap.Error(err))
		return nil, 0, err
	}

	// 2. Count total matching transactions
	total, err := uc.orderRepo.CountTransactionsByClientID(ctx, uc.db.Gorm, clientID, filter)
	if err != nil {
		log.Error("tenant transaction usecase FindAll count failed", zap.Error(err))
		return nil, 0, err
	}

	// 3. Convert to response models
	responses := converter.TransactionsToResponse(orders)

	// 4. Replace raw receipt URLs with time-limited presigned URLs
	if uc.s3Client != nil {
		for i := range responses {
			if responses[i].ReceiptURL == "" {
				continue
			}
			// Reconstruct S3 key (same pattern used during upload)
			s3Key := fmt.Sprintf("receipts/%s/%s.pdf", clientID.String(), responses[i].ID.String())
			presignedURL, err := uc.s3Client.PresignGet(ctx, &pkg_s3.PresignInput{
				Key:       s3Key,
				ExpiresIn: receiptPresignExpiry,
			})
			if err != nil {
				log.Warn("tenant transaction: presign receipt URL failed, falling back to stored URL",
					zap.String("order_id", responses[i].ID.String()),
					zap.Error(err),
				)
				continue
			}
			responses[i].ReceiptURL = presignedURL
		}
	}

	return responses, total, nil
}
