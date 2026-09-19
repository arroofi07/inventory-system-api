package main

import "context"

// Tahap adalah satu langkah migrasi dalam urutan FK (09 §3).
type Tahap interface {
	Nama() string
	Jalankan(ctx context.Context, sumber *Sumber, target *Target, lap *Laporan) error
}

type tahapFn struct {
	nama string
	run  func(ctx context.Context, sumber *Sumber, target *Target, lap *Laporan) error
}

func (t tahapFn) Nama() string { return t.nama }

func (t tahapFn) Jalankan(ctx context.Context, sumber *Sumber, target *Target, lap *Laporan) error {
	return t.run(ctx, sumber, target, lap)
}

// daftarTahap mengembalikan urutan tahap sesuai 09 §3.
func daftarTahap() []Tahap {
	return []Tahap{
		tahapFn{nama: "users", run: migrasiUsers},
		tahapFn{nama: "barang", run: migrasiBarang},
		tahapFn{nama: "barang_masuk", run: migrasiBarangMasuk},
		tahapFn{nama: "pelanggan", run: migrasiPelanggan},
		tahapFn{nama: "promos", run: migrasiPromo},
		tahapFn{nama: "transaksi_penjualan", run: migrasiTransaksiHeader},
		tahapFn{nama: "transaksi_detail", run: migrasiTransaksiDetail},
		tahapFn{nama: "transaksi_detail_promo", run: migrasiTransaksiDetailPromo},
		tahapFn{nama: "riwayat_pembayaran", run: migrasiRiwayatPembayaran},
		tahapFn{nama: "price_change_logs", run: migrasiPriceChangeLogs},
		tahapFn{nama: "stock_movements", run: bangunLedger},
		tahapFn{nama: "barang.stok_tersedia", run: isiStokTersedia},
		tahapFn{nama: "no_transaksi_seq", run: aturCounterNoTransaksi},
	}
}
