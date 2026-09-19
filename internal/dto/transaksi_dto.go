package dto

import (
	"fmt"
	"strings"
)

// TransaksiItemRequest satu baris item pada request pratinjau/store/add-items.
// Promo dirujuk by kode (bukan FK wajib); server men-resolve ke master aktif.
type TransaksiItemRequest struct {
	KodeItem string `json:"kode_item" validate:"required,max=64" example:"SHS001"`
	// Qty adalah jumlah yang ditagih (bukan total keluar gudang).
	Qty   int    `json:"qty" validate:"required,min=1" example:"10"`
	Harga string `json:"harga" validate:"required" example:"20000.00"`

	Disc1Persen string `json:"disc1_persen,omitempty" example:"10.00"`
	Disc2Persen string `json:"disc2_persen,omitempty" example:"0.00"`
	Disc3Persen string `json:"disc3_persen,omitempty" example:"0.00"`

	// KodePromos daftar kode promo yang diminta client. Opsional; server
	// memvalidasi keberlakuan (tanggal, SKU, qty, min amount).
	KodePromos []string `json:"kode_promos,omitempty" example:"PROMO2026090001"`

	// BarangMasukID opsional: paksa batch tertentu alih-alih FEFO.
	BarangMasukID *uint64 `json:"barang_masuk_id,omitempty"`
}

// TransaksiPratinjauRequest body POST /transaksi/pratinjau.
//
// Mode multi: isi items[] (min 1).
// Mode single: isi objek item di header request (tanpa items[]).
// Persistensi (SC-03+) selalu menulis baris transaksi_detail — tidak ada kolom item di header DB.
type TransaksiPratinjauRequest struct {
	KodePelanggan string                 `json:"kode_pelanggan" validate:"required,max=32" example:"0874"`
	Tanggal       string                 `json:"tanggal" validate:"required" example:"2026-09-04"`
	Item          *TransaksiItemRequest  `json:"item,omitempty"`
	Items         []TransaksiItemRequest `json:"items,omitempty"`

	Disc1Persen string `json:"disc1_persen,omitempty" example:"5.00"`
	Disc2Persen string `json:"disc2_persen,omitempty"`
	Disc3Persen string `json:"disc3_persen,omitempty"`
	PPNPersen   string `json:"ppn_persen,omitempty" example:"11.00"`
}

// TransaksiCreateRequest body POST /transaksi (simpan pending).
type TransaksiCreateRequest struct {
	KodePelanggan string                 `json:"kode_pelanggan" validate:"required,max=32" example:"0874"`
	Tanggal       string                 `json:"tanggal" validate:"required" example:"2026-09-04"`
	Area          string                 `json:"area" validate:"required,max=100" example:"Bandung Timur"`
	Item          *TransaksiItemRequest  `json:"item,omitempty"`
	Items         []TransaksiItemRequest `json:"items,omitempty"`

	Disc1Persen string `json:"disc1_persen,omitempty" example:"5.00"`
	Disc2Persen string `json:"disc2_persen,omitempty"`
	Disc3Persen string `json:"disc3_persen,omitempty"`
	PPNPersen   string `json:"ppn_persen,omitempty" example:"11.00"`

	NominalDibayar       string  `json:"nominal_dibayar,omitempty" example:"0.00"`
	TanggalJatuhTempo    *string `json:"tanggal_jatuh_tempo,omitempty" example:"2026-10-04"`
	KeteranganPembayaran string  `json:"keterangan_pembayaran,omitempty" validate:"omitempty,max=500"`
}

// TransaksiAddItemsRequest body POST /transaksi/{id}/items (hanya pending).
type TransaksiAddItemsRequest struct {
	Item  *TransaksiItemRequest  `json:"item,omitempty"`
	Items []TransaksiItemRequest `json:"items,omitempty"`
}

// CekStokItemRequest satu baris permintaan cek stok (SC-05). Harga tidak diperlukan.
type CekStokItemRequest struct {
	KodeItem      string  `json:"kode_item" validate:"required,max=64" example:"SHS001"`
	Qty           int     `json:"qty" validate:"required,min=1" example:"10"`
	BarangMasukID *uint64 `json:"barang_masuk_id,omitempty"`
}

// TransaksiCekStokRequest body POST /transaksi/cek-stok.
// Soft-check saja — bukan jaminan; jaminan hanya di approval (SC-10).
type TransaksiCekStokRequest struct {
	Items []CekStokItemRequest `json:"items" validate:"required,min=1"`
}

// CekStokBatchResponse salinan batch ledger yang relevan.
type CekStokBatchResponse struct {
	BarangMasukID uint64 `json:"barang_masuk_id" example:"305"`
	NoBatch       string `json:"no_batch" example:"B-003"`
	Exp           string `json:"exp" example:"2026-03-31"`
	QtyTersedia   int    `json:"qty_tersedia" example:"48"`
}

// CekStokItemResponse hasil cek per baris permintaan.
type CekStokItemResponse struct {
	KodeItem     string                 `json:"kode_item"`
	NamaItem     string                 `json:"nama_item"`
	QtyDiminta   int                    `json:"qty_diminta"`
	StokTersedia int                    `json:"stok_tersedia"`
	StokCukup    bool                   `json:"stok_cukup"`
	Batch        []CekStokBatchResponse `json:"batch"`
}

// TransaksiCekStokResponse data POST /transaksi/cek-stok.
type TransaksiCekStokResponse struct {
	Items          []CekStokItemResponse `json:"items"`
	SemuaStokCukup bool                  `json:"semua_stok_cukup"`
	Catatan        string                `json:"catatan"`
}

// TransaksiCekStokOKResponse envelope 200 cek stok.
type TransaksiCekStokOKResponse struct {
	Data TransaksiCekStokResponse `json:"data"`
}

// NormalisasiItems menggabungkan mode single (item) dan multi (items[]) menjadi slice kanonik.
func (r TransaksiPratinjauRequest) NormalisasiItems() ([]TransaksiItemRequest, error) {
	return normalisasiItems(r.Item, r.Items)
}

// NormalisasiItems menggabungkan mode single dan multi.
func (r TransaksiCreateRequest) NormalisasiItems() ([]TransaksiItemRequest, error) {
	return normalisasiItems(r.Item, r.Items)
}

// NormalisasiItems menggabungkan mode single dan multi.
func (r TransaksiAddItemsRequest) NormalisasiItems() ([]TransaksiItemRequest, error) {
	return normalisasiItems(r.Item, r.Items)
}

func normalisasiItems(single *TransaksiItemRequest, items []TransaksiItemRequest) ([]TransaksiItemRequest, error) {
	if len(items) > 0 {
		if single != nil {
			return nil, fmt.Errorf("kirim item (single) atau items[] (multi), bukan keduanya")
		}
		for i, it := range items {
			if err := validasiItemRequest(it, fmt.Sprintf("items[%d]", i)); err != nil {
				return nil, err
			}
		}
		return items, nil
	}

	if single == nil {
		return nil, fmt.Errorf("items wajib diisi, atau kirim objek item untuk mode single")
	}
	if err := validasiItemRequest(*single, "item"); err != nil {
		return nil, err
	}
	return []TransaksiItemRequest{*single}, nil
}

func validasiItemRequest(it TransaksiItemRequest, prefix string) error {
	if strings.TrimSpace(it.KodeItem) == "" {
		return fmt.Errorf("%s.kode_item wajib diisi", prefix)
	}
	if it.Qty < 1 {
		return fmt.Errorf("%s.qty minimal 1", prefix)
	}
	if strings.TrimSpace(it.Harga) == "" {
		return fmt.Errorf("%s.harga wajib diisi", prefix)
	}
	return nil
}

// IsMultiItem true bila lebih dari satu baris setelah normalisasi.
func IsMultiItem(items []TransaksiItemRequest) bool {
	return len(items) > 1
}

// --- Response ---

// PromoTerapanResponse snapshot promo yang diterapkan pada baris.
type PromoTerapanResponse struct {
	PromoID     *uint64 `json:"promo_id,omitempty" example:"3"`
	KodePromo   string  `json:"kode_promo" example:"PROMO2026090001"`
	NamaPromo   string  `json:"nama_promo" example:"Beli 10 Gratis 1"`
	TipePromo   string  `json:"tipe_promo,omitempty" example:"buy_x_get_y"`
	QtyBonus    int     `json:"qty_bonus" example:"1"`
	NilaiDiskon string  `json:"nilai_diskon" example:"0.00"`
}

// AlokasiBatchResponse rencana/hasil alokasi FEFO per baris.
type AlokasiBatchResponse struct {
	BarangMasukID uint64  `json:"barang_masuk_id" example:"305"`
	NoBatch       string  `json:"no_batch" example:"B-003"`
	Exp           *string `json:"exp,omitempty" example:"2026-03-31"`
	Qty           int     `json:"qty" example:"11"`
}

// TransaksiItemResponse baris detail transaksi (selalu dari transaksi_detail).
//
// Field jumlah = qty yang ditagih. Field total_qty_keluar = jumlah + qty_promo
// (qty yang mengurangi stok). Keduanya wajib ada agar tidak mengulang ambiguitas
// kolom "jumlah" sistem lama.
type TransaksiItemResponse struct {
	ID            uint64  `json:"id,omitempty" example:"1"`
	Urutan        uint16  `json:"urutan" example:"1"`
	KodeItem      string  `json:"kode_item" example:"SHS001"`
	NamaItem      string  `json:"nama_item" example:"Shisena Hand Body Lotion 100ml"`
	Satuan        string  `json:"satuan" example:"PCS"`
	BarangMasukID *uint64 `json:"barang_masuk_id,omitempty"`
	BatchNumber   *string `json:"batch_number,omitempty" example:"B-003"`
	ExpiryDate    *string `json:"expiry_date,omitempty" example:"2026-03-31"`

	// Jumlah qty yang ditagih (bukan qty keluar gudang).
	Jumlah int `json:"jumlah" example:"10"`
	// QtyPromo total bonus dari semua promo baris.
	QtyPromo int `json:"qty_promo" example:"1"`
	// TotalQtyKeluar = jumlah + qty_promo; inilah yang mengurangi stok.
	TotalQtyKeluar int `json:"total_qty_keluar" example:"11"`

	Harga          string `json:"harga" example:"20000.00"`
	Subtotal       string `json:"subtotal" example:"200000.00"`
	Disc1Persen    string `json:"disc1_persen" example:"10.00"`
	Disc2Persen    string `json:"disc2_persen" example:"0.00"`
	Disc3Persen    string `json:"disc3_persen" example:"0.00"`
	TotalAfterDisc string `json:"total_after_disc" example:"180000.00"`
	HPPSnapshot    string `json:"hpp_snapshot" example:"15400.00"`

	PromoDiterapkan []PromoTerapanResponse `json:"promo_diterapkan,omitempty"`
}

// PratinjauItemResponse baris hasil pratinjau (tanpa persist).
type PratinjauItemResponse struct {
	TransaksiItemResponse
	DiskonPromo   string                 `json:"diskon_promo" example:"0.00"`
	AlokasiBatch  []AlokasiBatchResponse `json:"alokasi_batch,omitempty"`
	StokCukup     bool                   `json:"stok_cukup" example:"true"`
	StokTersedia  int                    `json:"stok_tersedia" example:"148"`
}

// PratinjauRingkasan totals header pratinjau.
type PratinjauRingkasan struct {
	JumlahItem      int    `json:"jumlah_item" example:"2"`
	TotalQtyDitagih int    `json:"total_qty_ditagih" example:"15"`
	TotalQtyKeluar  int    `json:"total_qty_keluar" example:"16"`
	GrandTotal      string `json:"grand_total" example:"330000.00"`
	Total           string `json:"total" example:"313500.00"`
	PPNNominal      string `json:"ppn_nominal" example:"34485.00"`
	TotalAkhir      string `json:"total_akhir" example:"347985.00"`
}

// PratinjauTransaksiResponse data POST /transaksi/pratinjau.
type PratinjauTransaksiResponse struct {
	Items          []PratinjauItemResponse `json:"items"`
	Ringkasan      PratinjauRingkasan      `json:"ringkasan"`
	SemuaStokCukup bool                    `json:"semua_stok_cukup" example:"true"`
	Peringatan     []string                `json:"peringatan"`
}

// UserRingkasResponse referensi user ringkas di transaksi.
type UserRingkasResponse struct {
	ID   uint64 `json:"id" example:"2"`
	Name string `json:"name" example:"Sales Bandung"`
}

// TransaksiResponse detail transaksi (GET /transaksi/{id}, hasil POST store/add-items).
//
// Snapshot pelanggan (nama, alamat, channel) tersimpan di header.
// Semua baris item — termasuk single-item — ada di items (sumber: transaksi_detail).
// is_multi_item = true bila len(items) > 1.
type TransaksiResponse struct {
	ID            uint64  `json:"id" example:"42"`
	NoTransaksi   *string `json:"no_transaksi" example:"000148"`
	Tanggal       string  `json:"tanggal" example:"2026-09-04"`
	Periode       string  `json:"periode" example:"2026-09"`
	KodePelanggan string  `json:"kode_pelanggan" example:"0874"`
	NamaPelanggan string  `json:"nama_pelanggan" example:"Toko Maju"`
	Alamat        string  `json:"alamat" example:"Jl. Merdeka 1"`
	ChannelOutlet string  `json:"channel_outlet" example:"General Trade"`
	Area          string  `json:"area" example:"Bandung Timur"`
	IsMultiItem   bool    `json:"is_multi_item" example:"true"`

	Disc1Persen string `json:"disc1_persen" example:"5.00"`
	Disc2Persen string `json:"disc2_persen" example:"0.00"`
	Disc3Persen string `json:"disc3_persen" example:"0.00"`
	PPNPersen   string `json:"ppn_persen" example:"11.00"`
	Total       string `json:"total" example:"313500.00"`
	PPNNominal  string `json:"ppn_nominal" example:"34485.00"`
	TotalAkhir  string `json:"total_akhir" example:"347985.00"`

	JumlahItem      int `json:"jumlah_item" example:"2"`
	TotalQtyDitagih int `json:"total_qty_ditagih" example:"15"`
	TotalQtyKeluar  int `json:"total_qty_keluar" example:"16"`

	StatusApproval    string  `json:"status_approval" example:"pending" enums:"pending,approved,rejected"`
	ApprovedAt        *string `json:"approved_at,omitempty"`
	ApprovalNotes     *string `json:"approval_notes,omitempty"`
	Approver          *UserRingkasResponse `json:"approver,omitempty"`
	FulfillmentStatus string  `json:"fulfillment_status" example:"awaiting_approval"`

	StatusPembayaran          string  `json:"status_pembayaran" example:"hutang" enums:"lunas,hutang,sebagian"`
	JumlahDibayar             string  `json:"jumlah_dibayar" example:"0.00"`
	SisaHutang                string  `json:"sisa_hutang" example:"347985.00"`
	TanggalJatuhTempo         *string `json:"tanggal_jatuh_tempo,omitempty" example:"2026-10-04"`
	TanggalPembayaranTerakhir *string `json:"tanggal_pembayaran_terakhir,omitempty"`
	KeteranganPembayaran      *string `json:"keterangan_pembayaran,omitempty"`

	FakturDicetakAt *string              `json:"faktur_dicetak_at,omitempty"`
	Sales           *UserRingkasResponse `json:"sales,omitempty"`
	CreatedAt       string               `json:"created_at,omitempty"`
	UpdatedAt       string               `json:"updated_at,omitempty"`

	Items []TransaksiItemResponse `json:"items"`
}

// Envelope konkret untuk swag (generic Envelope[T] tidak selalu ter-parse).

// PratinjauTransaksiOKResponse envelope 200 pratinjau.
type PratinjauTransaksiOKResponse struct {
	Data PratinjauTransaksiResponse `json:"data"`
}

// TransaksiOKResponse envelope 200/201 transaksi.
type TransaksiOKResponse struct {
	Data TransaksiResponse `json:"data"`
}

// TransaksiListQuery filter GET /transaksi (SC-07).
type TransaksiListQuery struct {
	ListQuery
	StatusApproval   string  `form:"status_approval"`
	StatusPembayaran string  `form:"status_pembayaran"`
	DateFrom         string  `form:"date_from"`
	DateTo           string  `form:"date_to"`
	SalesID          *uint64 `form:"sales_id"`
	ChannelOutlet    string  `form:"channel_outlet"`
	// KecukupanStok: "cukup" | "kurang" — dihitung dari ledger vs total_qty_keluar (SC-09).
	KecukupanStok string `form:"kecukupan_stok"`
}

// StokKurangItem rincian 409 STOK_TIDAK_CUKUP saat approval.
type StokKurangItem struct {
	KodeItem string `json:"kode_item"`
	NamaItem string `json:"nama_item"`
	Diminta  int    `json:"diminta"`
	Tersedia int    `json:"tersedia"`
}

// ApproveRequest body POST /transaksi/{id}/approve.
type ApproveRequest struct {
	ApprovalNotes string `json:"approval_notes" validate:"omitempty,max=2000"`
}

// RejectRequest body POST /transaksi/{id}/reject.
type RejectRequest struct {
	ApprovalNotes string `json:"approval_notes" validate:"required,min=1,max=2000"`
}

// BulkApproveRequest body POST /transaksi/approvals/bulk.
type BulkApproveRequest struct {
	TransaksiIDs  []uint64 `json:"transaksi_ids" validate:"required,min=1,max=100"`
	ApprovalNotes string   `json:"approval_notes" validate:"omitempty,max=2000"`
}

// BulkApproveItemHasil satu ID dalam bulk.
type BulkApproveItemHasil struct {
	ID          uint64            `json:"id"`
	OK          bool              `json:"ok"`
	NoTransaksi *string           `json:"no_transaksi,omitempty"`
	Code        string            `json:"code,omitempty"`
	Message     string            `json:"message,omitempty"`
	Details     []StokKurangItem  `json:"details,omitempty"`
}

// BulkApproveResponse hasil bulk.
type BulkApproveResponse struct {
	Berhasil []BulkApproveItemHasil `json:"berhasil"`
	Gagal    []BulkApproveItemHasil `json:"gagal"`
	Ringkasan struct {
		Total    int `json:"total"`
		Berhasil int `json:"berhasil"`
		Gagal    int `json:"gagal"`
	} `json:"ringkasan"`
}

// PergerakanStokItem hasil approve (movement yang ditulis).
type PergerakanStokItem struct {
	KodeItem     string `json:"kode_item"`
	NoBatch      string `json:"no_batch"`
	Qty          int    `json:"qty"`
	SaldoSetelah int    `json:"saldo_setelah"`
}

// HasilApprovalResponse respons approve sukses.
type HasilApprovalResponse struct {
	ID             uint64               `json:"id"`
	NoTransaksi    string               `json:"no_transaksi"`
	StatusApproval string               `json:"status_approval"`
	ApprovedAt     string               `json:"approved_at"`
	ApprovedBy     uint64               `json:"approved_by"`
	TotalAkhir     string               `json:"total_akhir"`
	PergerakanStok []PergerakanStokItem `json:"pergerakan_stok"`
}

// KetersediaanSKUBaris baris tabel kecukupan (SC-09).
type KetersediaanSKUBaris struct {
	KodeItem     string `json:"kode_item"`
	NamaItem     string `json:"nama_item"`
	Diminta      int    `json:"diminta"`
	Tersedia     int    `json:"tersedia"`
	StokCukup    bool   `json:"stok_cukup"`
	BersaingIDs  []uint64 `json:"bersaing_ids"`
}

// TransaksiBersaingItem pending lain yang memakai SKU sama.
type TransaksiBersaingItem struct {
	ID            uint64  `json:"id"`
	KodePelanggan string  `json:"kode_pelanggan"`
	NamaPelanggan string  `json:"nama_pelanggan"`
	TotalAkhir    string  `json:"total_akhir"`
	Tanggal       string  `json:"tanggal"`
	KodeItems     []string `json:"kode_items"`
}

// KetersediaanStokResponse GET /transaksi/{id}/ketersediaan-stok.
type KetersediaanStokResponse struct {
	TransaksiID    uint64                   `json:"transaksi_id"`
	SemuaStokCukup bool                     `json:"semua_stok_cukup"`
	SKU            []KetersediaanSKUBaris   `json:"sku"`
	Bersaing       []TransaksiBersaingItem  `json:"bersaing"`
}

// TransaksiListItem baris daftar (tanpa detail items).
type TransaksiListItem struct {
	ID               uint64  `json:"id"`
	NoTransaksi      *string `json:"no_transaksi"`
	Tanggal          string  `json:"tanggal"`
	KodePelanggan    string  `json:"kode_pelanggan"`
	NamaPelanggan    string  `json:"nama_pelanggan"`
	ChannelOutlet    string  `json:"channel_outlet"`
	Area             string  `json:"area"`
	IsMultiItem      bool    `json:"is_multi_item"`
	JumlahItem       int     `json:"jumlah_item"`
	TotalQtyDitagih  int     `json:"total_qty_ditagih"`
	TotalQtyKeluar   int     `json:"total_qty_keluar"`
	Total            string  `json:"total"`
	PPNNominal       string  `json:"ppn_nominal"`
	TotalAkhir       string  `json:"total_akhir"`
	StatusApproval   string  `json:"status_approval"`
	StatusPembayaran string  `json:"status_pembayaran"`
	SalesID          *uint64 `json:"sales_id,omitempty"`
	// StokCukup diisi untuk pending (SC-09 indikator antrian); null bila tidak dihitung.
	StokCukup *bool  `json:"stok_cukup,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}
