package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// LaporanAudit adalah hasil --audit-sumber / --bersihkan-sumber (SF-02).
type LaporanAudit struct {
	Dijalankan   time.Time
	Sumber       string
	Mode         string // AUDIT | RENCANA | BERSIH
	Diterapkan   bool
	Catatan      []string
	DupKTP       []TemuanDupKTP
	DupBatch     []TemuanDupBatch
	ChannelNorm  []TemuanChannelNorm
	ChannelGagal []TemuanChannelUnmapped
	I7           []TemuanI7
	I8           []TemuanI8
}

type TemuanDupKTP struct {
	NoKTP         string
	PertahankanID uint64
	NullkanIDs    []uint64
}

type TemuanDupBatch struct {
	BarangID      uint64
	NoBatch       string
	NoFakturAsal  string
	PertahankanID uint64
	Ubah          []UbahFaktur
}

type UbahFaktur struct {
	ID           uint64
	NoFakturBaru string
}

type TemuanChannelNorm struct {
	ID   uint64
	Dari string
	Ke   string
}

type TemuanChannelUnmapped struct {
	ID    uint64
	Nilai string
}

type TemuanI7 struct {
	ID              uint64
	PerluNomor      bool
	PerluApprovedAt bool
	NomorBaru       string
}

type TemuanI8 struct {
	ID          uint64
	NoTransaksi string
}

func kunciBatch(barangID uint64, batch, faktur string) string {
	return fmt.Sprintf("%d\x00%s\x00%s", barangID, batch, faktur)
}

func rencanaDupKTP(grup map[string][]uint64) []TemuanDupKTP {
	out := make([]TemuanDupKTP, 0)
	for ktp, ids := range grup {
		if len(ids) < 2 {
			continue
		}
		cp := append([]uint64(nil), ids...)
		sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
		out = append(out, TemuanDupKTP{NoKTP: ktp, PertahankanID: cp[0], NullkanIDs: cp[1:]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].NoKTP < out[j].NoKTP })
	return out
}

func rencanaDupBatch(baris []barisBatch, existing map[string]uint64) []TemuanDupBatch {
	grup := map[string][]barisBatch{}
	order := []string{}
	for _, b := range baris {
		k := kunciBatch(b.BarangID, b.NoBatch, b.NoFaktur)
		if _, ok := grup[k]; !ok {
			order = append(order, k)
		}
		grup[k] = append(grup[k], b)
	}
	out := make([]TemuanDupBatch, 0)
	terpakai := map[string]uint64{}
	for k, v := range existing {
		terpakai[k] = v
	}
	for _, k := range order {
		anggota := grup[k]
		if len(anggota) < 2 {
			continue
		}
		sort.Slice(anggota, func(i, j int) bool { return anggota[i].ID < anggota[j].ID })
		t := TemuanDupBatch{
			BarangID:      anggota[0].BarangID,
			NoBatch:       anggota[0].NoBatch,
			NoFakturAsal:  anggota[0].NoFaktur,
			PertahankanID: anggota[0].ID,
		}
		kali := 1
		for _, lain := range anggota[1:] {
			kali++
			baru := suffixFakturUnik(lain.BarangID, lain.NoBatch, anggota[0].NoFaktur, lain.ID, kali, terpakai)
			terpakai[kunciBatch(lain.BarangID, lain.NoBatch, baru)] = lain.ID
			t.Ubah = append(t.Ubah, UbahFaktur{ID: lain.ID, NoFakturBaru: baru})
		}
		out = append(out, t)
	}
	return out
}

func suffixFakturUnik(barangID uint64, batch, fakturAsal string, id uint64, kaliKe int, terpakai map[string]uint64) string {
	for {
		calon := suffixFaktur(fakturAsal, kaliKe)
		k := kunciBatch(barangID, batch, calon)
		pemilik, ada := terpakai[k]
		if !ada || pemilik == id {
			return calon
		}
		kaliKe++
	}
}

func isiNomorI7(temuan []TemuanI7, last int64) []TemuanI7 {
	n := last
	out := make([]TemuanI7, len(temuan))
	copy(out, temuan)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	for i := range out {
		if !out[i].PerluNomor {
			continue
		}
		n++
		out[i].NomorBaru = fmt.Sprintf("%06d", n)
	}
	return out
}

type barisBatch struct {
	ID       uint64
	BarangID uint64
	NoBatch  string
	NoFaktur string
}

func auditSumber(ctx context.Context, sumber *Sumber) (*LaporanAudit, error) {
	lap := &LaporanAudit{
		Dijalankan: time.Now(),
		Sumber:     sumber.Label(),
		Mode:       "AUDIT",
	}
	if err := auditDupKTP(ctx, sumber, lap); err != nil {
		return nil, err
	}
	if err := auditDupBatch(ctx, sumber, lap); err != nil {
		return nil, err
	}
	if err := auditChannel(ctx, sumber, lap); err != nil {
		return nil, err
	}
	if err := auditI7I8(ctx, sumber, lap); err != nil {
		return nil, err
	}
	return lap, nil
}

func auditDupKTP(ctx context.Context, sumber *Sumber, lap *LaporanAudit) error {
	if !sumber.PunyaTabel(ctx, "users") {
		lap.Catatan = append(lap.Catatan, "tabel users tidak ada; skip duplikat no_ktp")
		return nil
	}
	if !sumber.PunyaKolom(ctx, "users", "no_ktp") {
		lap.Catatan = append(lap.Catatan, "kolom users.no_ktp tidak ada; skip")
		return nil
	}
	rows, err := sumber.QueryContext(ctx, `
		SELECT id, no_ktp FROM users
		WHERE no_ktp IS NOT NULL AND TRIM(no_ktp) <> ''
		ORDER BY id`)
	if err != nil {
		return fmt.Errorf("audit no_ktp: %w", err)
	}
	defer rows.Close()
	grup := map[string][]uint64{}
	for rows.Next() {
		var id uint64
		var ktp string
		if err := rows.Scan(&id, &ktp); err != nil {
			return err
		}
		key := strings.TrimSpace(ktp)
		grup[key] = append(grup[key], id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	lap.DupKTP = rencanaDupKTP(grup)
	return nil
}

func auditDupBatch(ctx context.Context, sumber *Sumber, lap *LaporanAudit) error {
	if !sumber.PunyaTabel(ctx, "barang_masuk") {
		lap.Catatan = append(lap.Catatan, "tabel barang_masuk tidak ada; skip duplikat batch")
		return nil
	}
	for _, kol := range []string{"barang_id", "no_batch", "no_faktur"} {
		if !sumber.PunyaKolom(ctx, "barang_masuk", kol) {
			lap.Catatan = append(lap.Catatan, "kolom barang_masuk."+kol+" tidak ada; skip duplikat batch")
			return nil
		}
	}
	rows, err := sumber.QueryContext(ctx, `
		SELECT id, barang_id, no_batch, no_faktur FROM barang_masuk ORDER BY id`)
	if err != nil {
		return fmt.Errorf("audit batch: %w", err)
	}
	defer rows.Close()
	var baris []barisBatch
	existing := map[string]uint64{}
	for rows.Next() {
		var b barisBatch
		if err := rows.Scan(&b.ID, &b.BarangID, &b.NoBatch, &b.NoFaktur); err != nil {
			return err
		}
		b.NoBatch = strings.TrimSpace(b.NoBatch)
		b.NoFaktur = strings.TrimSpace(b.NoFaktur)
		baris = append(baris, b)
		existing[kunciBatch(b.BarangID, b.NoBatch, b.NoFaktur)] = b.ID
	}
	if err := rows.Err(); err != nil {
		return err
	}
	lap.DupBatch = rencanaDupBatch(baris, existing)
	return nil
}

func auditChannel(ctx context.Context, sumber *Sumber, lap *LaporanAudit) error {
	if !sumber.PunyaTabel(ctx, "pelanggan") {
		lap.Catatan = append(lap.Catatan, "tabel pelanggan tidak ada; skip channel")
		return nil
	}
	if !sumber.PunyaKolom(ctx, "pelanggan", "channel_outlet") {
		lap.Catatan = append(lap.Catatan, "kolom pelanggan.channel_outlet tidak ada; skip")
		return nil
	}
	rows, err := sumber.QueryContext(ctx, `SELECT id, channel_outlet FROM pelanggan ORDER BY id`)
	if err != nil {
		return fmt.Errorf("audit channel: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id uint64
		var raw sql.NullString
		if err := rows.Scan(&id, &raw); err != nil {
			return err
		}
		nilai := strings.TrimSpace(strNS(raw))
		ch, ok := normalkanChannel(nilai)
		if !ok {
			lap.ChannelGagal = append(lap.ChannelGagal, TemuanChannelUnmapped{ID: id, Nilai: nilai})
			continue
		}
		kanon := string(ch)
		if nilai != kanon {
			lap.ChannelNorm = append(lap.ChannelNorm, TemuanChannelNorm{ID: id, Dari: nilai, Ke: kanon})
		}
	}
	return rows.Err()
}

func auditI7I8(ctx context.Context, sumber *Sumber, lap *LaporanAudit) error {
	if !sumber.PunyaTabel(ctx, "transaksi_penjualan") {
		lap.Catatan = append(lap.Catatan, "tabel transaksi_penjualan tidak ada; skip I7/I8")
		return nil
	}
	if !sumber.PunyaKolom(ctx, "transaksi_penjualan", "status_approval") {
		lap.Catatan = append(lap.Catatan, "kolom status_approval tidak ada; skip I7/I8")
		return nil
	}
	punyaNomor := sumber.PunyaKolom(ctx, "transaksi_penjualan", "no_transaksi")
	punyaApprovedAt := sumber.PunyaKolom(ctx, "transaksi_penjualan", "approved_at")
	exprNomor := "NULL"
	if punyaNomor {
		exprNomor = "no_transaksi"
	}
	exprApproved := "NULL"
	if punyaApprovedAt {
		exprApproved = "approved_at"
	}

	q := fmt.Sprintf(`
		SELECT id, status_approval, %s, %s
		FROM transaksi_penjualan
		ORDER BY id`, exprNomor, exprApproved)
	rows, err := sumber.QueryContext(ctx, q)
	if err != nil {
		return fmt.Errorf("audit I7/I8: %w", err)
	}
	defer rows.Close()

	var i7 []TemuanI7
	var last int64
	for rows.Next() {
		var id uint64
		var statusNS sql.NullString
		var nomor sql.NullString
		var approved sql.NullTime
		if err := rows.Scan(&id, &statusNS, &nomor, &approved); err != nil {
			return err
		}
		status := statusApproval(strNS(statusNS))
		nomorStr := strings.TrimSpace(strNS(nomor))
		if status == "approved" {
			if n, ok := parseNomorMurni(nomorStr); ok && n > last {
				last = n
			}
			t := TemuanI7{ID: id}
			if punyaNomor && nomorStr == "" {
				t.PerluNomor = true
			}
			if punyaApprovedAt && !approved.Valid {
				t.PerluApprovedAt = true
			}
			if t.PerluNomor || t.PerluApprovedAt {
				i7 = append(i7, t)
			}
			continue
		}
		if punyaNomor && nomorStr != "" {
			lap.I8 = append(lap.I8, TemuanI8{ID: id, NoTransaksi: nomorStr})
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	lap.I7 = isiNomorI7(i7, last)
	return nil
}

func parseNomorMurni(s string) (int64, bool) {
	if s == "" {
		return 0, false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	var n int64
	for _, r := range s {
		n = n*10 + int64(r-'0')
	}
	return n, true
}

func terapkanBersih(ctx context.Context, sumber *Sumber, lap *LaporanAudit) error {
	tx, err := sumber.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, d := range lap.DupKTP {
		for _, id := range d.NullkanIDs {
			if _, err := tx.ExecContext(ctx, `UPDATE users SET no_ktp = NULL WHERE id = ?`, id); err != nil {
				return fmt.Errorf("nullkan no_ktp id=%d: %w", id, err)
			}
		}
	}
	for _, d := range lap.DupBatch {
		for _, u := range d.Ubah {
			if _, err := tx.ExecContext(ctx,
				`UPDATE barang_masuk SET no_faktur = ? WHERE id = ?`, u.NoFakturBaru, u.ID); err != nil {
				return fmt.Errorf("suffix faktur id=%d: %w", u.ID, err)
			}
		}
	}
	for _, c := range lap.ChannelNorm {
		if _, err := tx.ExecContext(ctx,
			`UPDATE pelanggan SET channel_outlet = ? WHERE id = ?`, c.Ke, c.ID); err != nil {
			return fmt.Errorf("channel id=%d: %w", c.ID, err)
		}
	}
	for _, r := range lap.I8 {
		if _, err := tx.ExecContext(ctx,
			`UPDATE transaksi_penjualan SET no_transaksi = NULL WHERE id = ?`, r.ID); err != nil {
			return fmt.Errorf("I8 id=%d: %w", r.ID, err)
		}
	}
	approvedExpr := "CURRENT_TIMESTAMP"
	if sumber.PunyaKolom(ctx, "transaksi_penjualan", "updated_at") &&
		sumber.PunyaKolom(ctx, "transaksi_penjualan", "created_at") {
		approvedExpr = "COALESCE(updated_at, created_at, CURRENT_TIMESTAMP)"
	}
	for _, r := range lap.I7 {
		if r.PerluNomor && r.NomorBaru != "" {
			if _, err := tx.ExecContext(ctx,
				`UPDATE transaksi_penjualan SET no_transaksi = ? WHERE id = ?`, r.NomorBaru, r.ID); err != nil {
				return fmt.Errorf("I7 nomor id=%d: %w", r.ID, err)
			}
		}
		if r.PerluApprovedAt {
			q := fmt.Sprintf(`UPDATE transaksi_penjualan SET approved_at = %s WHERE id = ? AND approved_at IS NULL`, approvedExpr)
			if _, err := tx.ExecContext(ctx, q, r.ID); err != nil {
				return fmt.Errorf("I7 approved_at id=%d: %w", r.ID, err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	lap.Diterapkan = true
	lap.Mode = "BERSIH"
	return nil
}

func (l *LaporanAudit) JumlahPerubahan() int {
	n := 0
	for _, d := range l.DupKTP {
		n += len(d.NullkanIDs)
	}
	for _, d := range l.DupBatch {
		n += len(d.Ubah)
	}
	n += len(l.ChannelNorm) + len(l.I8)
	for _, r := range l.I7 {
		if r.PerluNomor {
			n++
		}
		if r.PerluApprovedAt {
			n++
		}
	}
	return n
}

func (l *LaporanAudit) Format(w io.Writer) error {
	fmt.Fprintln(w, "=== Audit sumber Laravel (SF-02) ===")
	fmt.Fprintf(w, "Dijalankan : %s\n", l.Dijalankan.Format(time.RFC3339))
	fmt.Fprintf(w, "Sumber     : %s\n", l.Sumber)
	fmt.Fprintf(w, "Mode       : %s\n", l.Mode)
	if l.Diterapkan {
		fmt.Fprintln(w, "Diterapkan : ya (salinan sumber)")
	} else {
		fmt.Fprintln(w, "Diterapkan : tidak (hanya laporan)")
	}
	fmt.Fprintln(w)
	if len(l.Catatan) > 0 {
		fmt.Fprintln(w, "--- Catatan ---")
		for _, c := range l.Catatan {
			fmt.Fprintf(w, "- %s\n", c)
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "--- Duplikat no_ktp ---")
	if len(l.DupKTP) == 0 {
		fmt.Fprintln(w, "(tidak ada)")
	} else {
		for _, d := range l.DupKTP {
			fmt.Fprintf(w, "no_ktp=%s pertahankan id=%d nullkan %v\n", d.NoKTP, d.PertahankanID, d.NullkanIDs)
		}
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "--- Duplikat (barang_id, no_batch, no_faktur) ---")
	if len(l.DupBatch) == 0 {
		fmt.Fprintln(w, "(tidak ada)")
	} else {
		for _, d := range l.DupBatch {
			fmt.Fprintf(w, "barang_id=%d batch=%s faktur=%s pertahankan id=%d\n",
				d.BarangID, d.NoBatch, d.NoFakturAsal, d.PertahankanID)
			for _, u := range d.Ubah {
				fmt.Fprintf(w, "  id=%d → no_faktur %s\n", u.ID, u.NoFakturBaru)
			}
		}
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "--- Channel dinormalisasi ---")
	if len(l.ChannelNorm) == 0 {
		fmt.Fprintln(w, "(tidak ada)")
	} else {
		for _, c := range l.ChannelNorm {
			fmt.Fprintf(w, "[id=%d] %q → %q\n", c.ID, c.Dari, c.Ke)
		}
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "--- Channel tidak terpetakan (koreksi manual) ---")
	if len(l.ChannelGagal) == 0 {
		fmt.Fprintln(w, "(tidak ada)")
	} else {
		for _, c := range l.ChannelGagal {
			fmt.Fprintf(w, "[id=%d] %q\n", c.ID, c.Nilai)
		}
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "--- I7 approved tanpa no_transaksi / approved_at ---")
	if len(l.I7) == 0 {
		fmt.Fprintln(w, "(tidak ada)")
	} else {
		for _, r := range l.I7 {
			fmt.Fprintf(w, "[id=%d] perlu_nomor=%v nomor_baru=%s perlu_approved_at=%v\n",
				r.ID, r.PerluNomor, r.NomorBaru, r.PerluApprovedAt)
		}
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "--- I8 non-approved punya no_transaksi ---")
	if len(l.I8) == 0 {
		fmt.Fprintln(w, "(tidak ada)")
	} else {
		for _, r := range l.I8 {
			fmt.Fprintf(w, "[id=%d] no_transaksi=%s → NULL\n", r.ID, r.NoTransaksi)
		}
	}
	fmt.Fprintln(w)

	fmt.Fprintf(w, "RINGKASAN: %d perubahan otomatis, %d channel belum terpetakan\n",
		l.JumlahPerubahan(), len(l.ChannelGagal))
	if len(l.ChannelGagal) > 0 {
		fmt.Fprintln(w, "Channel tidak terpetakan tidak diubah otomatis (09 §4 tahap 4).")
	}
	return nil
}
