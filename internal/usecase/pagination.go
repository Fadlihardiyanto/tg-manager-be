package usecase

import "math"

// clampPagination membatasi page/limit supaya (page-1)*limit tidak overflow
// (offset negatif -> Postgres error -> 500) dan mengikuti konvensi repo
// (limit maksimal 100). Behavior-preserving untuk semua nilai wajar.
func clampPagination(page, limit int) (int, int) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	const maxPage = 1_000_000
	if page > maxPage {
		page = maxPage
	}
	return page, limit
}

// clampToInt konversi int64 → int tanpa silent wraparound di build 32-bit.
// Di build 64-bit (int = int64) ini identity — biaya nol.
func clampToInt(v int64) int {
	if v > math.MaxInt {
		return math.MaxInt
	}
	if v < math.MinInt {
		return math.MinInt
	}
	return int(v)
}
