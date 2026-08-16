package controller

// clampPage membatasi nilai page dari query param. Tanpa batas atas,
// (page-1)*limit overflow int → offset negatif/absurd → salah data atau 500.
// Mengikuti konvensi repo (max 1 juta halaman).
func clampPage(page int) int {
	if page < 1 {
		return 1
	}
	const maxPage = 1_000_000
	if page > maxPage {
		return maxPage
	}
	return page
}
