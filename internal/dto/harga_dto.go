package dto

// HargaMassalRequest body POST /barang/{id}/harga-massal.
type HargaMassalRequest struct {
	BatchIDs       []uint64 `json:"batch_ids" validate:"required,min=1,dive,min=1"`
	Harga          *string  `json:"harga"`
	DiscHPP1       *string  `json:"disc_hpp_1"`
	DiscHPP2       *string  `json:"disc_hpp_2"`
	DiscHPP3       *string  `json:"disc_hpp_3"`
	MarkupMTType   *string  `json:"markup_mt_type" validate:"omitempty,oneof=percent value"`
	MarkupMTAmount *string  `json:"markup_mt_amount"`
	MarkupGTType   *string  `json:"markup_gt_type" validate:"omitempty,oneof=percent value"`
	MarkupGTAmount *string  `json:"markup_gt_amount"`
	Keterangan     *string  `json:"keterangan" validate:"omitempty,max=500"`
}

// HargaMassalResponse hasil bulk update.
type HargaMassalResponse struct {
	BulkOperationID       string `json:"bulk_operation_id"`
	JumlahBatchDiperbarui int    `json:"jumlah_batch_diperbarui"`
	JumlahDilewati        int    `json:"jumlah_dilewati"`
}

// PriceChangeLogResponse ringkas riwayat harga.
type PriceChangeLogResponse struct {
	ID              uint64  `json:"id"`
	BarangMasukID   uint64  `json:"barang_masuk_id"`
	NoBatch         string  `json:"no_batch"`
	BulkOperationID string  `json:"bulk_operation_id"`
	OldHarga        *string `json:"old_harga,omitempty"`
	NewHarga        *string `json:"new_harga,omitempty"`
	OldHPP          *string `json:"old_hpp,omitempty"`
	NewHPP          *string `json:"new_hpp,omitempty"`
	OldHargaMT      *string `json:"old_harga_mt,omitempty"`
	NewHargaMT      *string `json:"new_harga_mt,omitempty"`
	OldHargaGT      *string `json:"old_harga_gt,omitempty"`
	NewHargaGT      *string `json:"new_harga_gt,omitempty"`
	Keterangan      *string `json:"keterangan,omitempty"`
	ChangedAt       string  `json:"changed_at"`
}

// RiwayatHargaQuery paginasi riwayat.
type RiwayatHargaQuery struct {
	ListQuery
}
