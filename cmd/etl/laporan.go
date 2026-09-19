package main

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// BarisGagal mencatat satu kegagalan per baris sumber (09 §2 prinsip 3).
type BarisGagal struct {
	Tahap string
	ID    uint64
	Pesan string
}

// Peringatan mencatat isu non-fatal yang perlu ditinjau.
type Peringatan struct {
	Tahap string
	ID    uint64
	Pesan string
}

// RingkasanTahap mengagregasi sukses/gagal/peringatan per tahap.
type RingkasanTahap struct {
	Nama       string
	Berhasil   int
	Gagal      int
	Peringatan int
}

// HasilVerifikasi adalah satu baris hasil V1–V9 (diisi SF-06).
type HasilVerifikasi struct {
	Kode       string
	Nama       string
	Status     string // LULUS | GAGAL | TINJAU | SKIP
	Detail     string
	WajibLulus bool
}

// BarisInflate transaksi historis dengan total_akhir yang tidak konsisten (11 §2.1).
type BarisInflate struct {
	ID          uint64
	NoTransaksi string
	Total       string
	PPNPersen   string
	TotalAkhir  string
	Seharusnya  string
	Selisih     string
}
type Laporan struct {
	mu sync.Mutex

	Dijalankan time.Time
	Selesai    time.Time
	Sumber     string
	Target     string
	Checksum   string

	urutan     []string
	ring       map[string]*RingkasanTahap
	gagal      []BarisGagal
	pering     []Peringatan
	verif      []HasilVerifikasi
	kesimpulan string
	inflate    []BarisInflate
}

func NewLaporan(sumber, target string) *Laporan {
	return &Laporan{
		Dijalankan: time.Now(),
		Sumber:     sumber,
		Target:     target,
		ring:       make(map[string]*RingkasanTahap),
	}
}

func (l *Laporan) pastikanTahap(nama string) *RingkasanTahap {
	if r, ok := l.ring[nama]; ok {
		return r
	}
	r := &RingkasanTahap{Nama: nama}
	l.ring[nama] = r
	l.urutan = append(l.urutan, nama)
	return r
}

// Berhasil menambah hitungan sukses untuk tahap.
func (l *Laporan) Berhasil(tahap string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pastikanTahap(tahap).Berhasil++
}

// Gagal mencatat baris gagal tanpa menghentikan seluruh ETL.
func (l *Laporan) Gagal(tahap string, id uint64, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pastikanTahap(tahap).Gagal++
	pesan := "unknown"
	if err != nil {
		pesan = err.Error()
	}
	l.gagal = append(l.gagal, BarisGagal{Tahap: tahap, ID: id, Pesan: pesan})
}

// Peringatan mencatat isu non-fatal.
func (l *Laporan) Peringatan(tahap string, id uint64, pesan string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pastikanTahap(tahap).Peringatan++
	l.pering = append(l.pering, Peringatan{Tahap: tahap, ID: id, Pesan: pesan})
}

// SetVerifikasi menyimpan hasil runner V1–V9.
func (l *Laporan) SetVerifikasi(hasil []HasilVerifikasi, kesimpulan string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.verif = append([]HasilVerifikasi(nil), hasil...)
	l.kesimpulan = kesimpulan
}

// CatatInflate menandai transaksi yang total_akhir-nya tidak dikoreksi (SF-04).
func (l *Laporan) CatatInflate(b BarisInflate) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.inflate = append(l.inflate, b)
}

// DaftarInflate salinan daftar inflate (untuk test / artefak terpisah).
func (l *Laporan) DaftarInflate() []BarisInflate {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]BarisInflate(nil), l.inflate...)
}

// SetChecksum menyimpan checksum target setelah ETL.
func (l *Laporan) SetChecksum(cs string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Checksum = cs
}

// Tutup menandai waktu selesai.
func (l *Laporan) Tutup() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Selesai = time.Now()
}

// TotalGagal mengembalikan jumlah baris gagal.
func (l *Laporan) TotalGagal() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, r := range l.ring {
		n += r.Gagal
	}
	return n
}

// Format menulis laporan teks sesuai 09 §7.
func (l *Laporan) Format(w io.Writer) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.Selesai.IsZero() {
		l.Selesai = time.Now()
	}
	durasi := l.Selesai.Sub(l.Dijalankan).Round(time.Second)

	if _, err := fmt.Fprintln(w, "=== Laporan ETL PKB Web ==="); err != nil {
		return err
	}
	fmt.Fprintf(w, "Dijalankan   : %s\n", l.Dijalankan.Format(time.RFC3339))
	fmt.Fprintf(w, "Sumber       : %s\n", l.Sumber)
	fmt.Fprintf(w, "Target       : %s\n", l.Target)
	fmt.Fprintf(w, "Durasi       : %s\n", formatDurasi(durasi))
	if l.Checksum != "" {
		fmt.Fprintf(w, "Checksum     : %s\n", l.Checksum)
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "--- Ringkasan per tahap ---")
	fmt.Fprintf(w, "%-28s %8s %8s %12s\n", "Tahap", "Berhasil", "Gagal", "Peringatan")
	for _, nama := range l.urutan {
		r := l.ring[nama]
		fmt.Fprintf(w, "%-28s %8d %8d %12d\n", r.Nama, r.Berhasil, r.Gagal, r.Peringatan)
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "--- Baris gagal (perlu tindakan) ---")
	if len(l.gagal) == 0 {
		fmt.Fprintln(w, "(tidak ada)")
	} else {
		for _, g := range l.gagal {
			fmt.Fprintf(w, "[%s id=%d]  %s\n", g.Tahap, g.ID, g.Pesan)
		}
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "--- Peringatan ---")
	if len(l.pering) == 0 {
		fmt.Fprintln(w, "(tidak ada)")
	} else {
		for _, p := range l.pering {
			if p.ID == 0 {
				fmt.Fprintf(w, "[%s]  %s\n", p.Tahap, p.Pesan)
			} else {
				fmt.Fprintf(w, "[%s id=%d]  %s\n", p.Tahap, p.ID, p.Pesan)
			}
		}
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "--- Transaksi total_akhir tidak konsisten (keuangan) ---")
	if len(l.inflate) == 0 {
		fmt.Fprintln(w, "(tidak ada)")
	} else {
		fmt.Fprintf(w, "%d transaksi dipertahankan apa adanya (jangan dikoreksi).\n", len(l.inflate))
		for _, b := range l.inflate {
			fmt.Fprintf(w, "[id=%d no=%s] total=%s ppn=%s%% total_akhir=%s seharusnya=%s selisih=%s\n",
				b.ID, b.NoTransaksi, b.Total, b.PPNPersen, b.TotalAkhir, b.Seharusnya, b.Selisih)
		}
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "--- Verifikasi paritas ---")
	if len(l.verif) == 0 {
		fmt.Fprintln(w, "(belum dijalankan — lihat SF-06)")
	} else {
		for _, v := range l.verif {
			line := fmt.Sprintf("%-4s %-28s %-6s", v.Kode, v.Nama, v.Status)
			if v.Detail != "" {
				line += "  " + v.Detail
			}
			fmt.Fprintln(w, line)
		}
	}
	fmt.Fprintln(w)

	if l.kesimpulan != "" {
		fmt.Fprintf(w, "KESIMPULAN: %s\n", l.kesimpulan)
	} else {
		fmt.Fprintln(w, "KESIMPULAN: kerangka ETL selesai; tahap migrasi diisi di SF-03..SF-05, verifikasi di SF-06.")
	}
	return nil
}

func formatDurasi(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d detik", int(d.Seconds()))
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%d menit %d detik", m, s)
}

// SnapshotRingkasan mengembalikan salinan ringkasan (untuk test).
func (l *Laporan) SnapshotRingkasan() []RingkasanTahap {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]RingkasanTahap, 0, len(l.urutan))
	for _, nama := range l.urutan {
		out = append(out, *l.ring[nama])
	}
	return out
}

// NamaTahapTerurut mengembalikan urutan tahap yang sudah dilaporkan.
func (l *Laporan) NamaTahapTerurut() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.urutan...)
}
