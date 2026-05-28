package entity

import (
	"github.com/jmoiron/sqlx"
	"gorm.io/gorm"
)

// Database holds both GORM (for Write/Transactional) and sqlx (for Read/Reporting) connections.
type Database struct {
	Gorm *gorm.DB
	Sqlx *sqlx.DB
}
