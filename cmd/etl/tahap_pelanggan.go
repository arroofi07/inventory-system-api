package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"app/internal/domain"
	"app/internal/pkg/money"
)

// Migrasi pelanggan — SF-03. Channel dinormalisasi ke enum; placeholder → NULL.
func migrasiPelanggan(ctx context.Context, sumber *Sumber, target *Target, lap *Laporan) error {
	if skipSumber(sumber, lap, "pelanggan") {
		return nil
	}
	if !sumber.PunyaTabel(ctx, "pelanggan") {
		return fmt.Errorf("tabel sumber pelanggan tidak ada")
	}

	q := fmt.Sprintf(`
		SELECT
			id,
			%s, %s, %s, %s,
			%s, %s, %s,
			%s, %s, %s, %s,
			%s, %s, %s, %s, %s,
			%s, %s, %s, %s,
			%s, %s,
			%s, %s
		FROM pelanggan
		ORDER BY id`,
		sumber.ExprKolom(ctx, "pelanggan", "kode_pelanggan", "kode_pelanggan", "''"),
		sumber.ExprKolom(ctx, "pelanggan", "nama_pelanggan", "nama_pelanggan", "''"),
		sumber.ExprKolom(ctx, "pelanggan", "tgl_registrasi", "tgl_registrasi", "NULL"),
		sumber.ExprKolom(ctx, "pelanggan", "phone", "phone", "''"),
		sumber.ExprKolom(ctx, "pelanggan", "npwp_nik", "npwp_nik", "NULL"),
		sumber.ExprKolom(ctx, "pelanggan", "nama_pemilik_npwp_nik", "nama_pemilik_npwp_nik", "NULL"),
		sumber.ExprKolom(ctx, "pelanggan", "alamat_npwp_nik", "alamat_npwp_nik", "NULL"),
		sumber.ExprKolom(ctx, "pelanggan", "territory", "territory", "''"),
		sumber.ExprKolom(ctx, "pelanggan", "distrik", "distrik", "''"),
		sumber.ExprKolom(ctx, "pelanggan", "alamat_toko", "alamat_toko", "''"),
		sumber.ExprKolom(ctx, "pelanggan", "rt_rw", "rt_rw", "NULL"),
		sumber.ExprKolom(ctx, "pelanggan", "provinsi", "provinsi", "''"),
		sumber.ExprKolom(ctx, "pelanggan", "kabupaten", "kabupaten", "''"),
		sumber.ExprKolom(ctx, "pelanggan", "kecamatan", "kecamatan", "''"),
		sumber.ExprKolom(ctx, "pelanggan", "kelurahan", "kelurahan", "''"),
		sumber.ExprKolom(ctx, "pelanggan", "kode_pos", "kode_pos", "NULL"),
		sumber.ExprKolom(ctx, "pelanggan", "channel_outlet", "channel_outlet", "''"),
		sumber.ExprKolom(ctx, "pelanggan", "alamat_pengantaran_barang", "alamat_pengantaran_barang", "NULL"),
		sumber.ExprKolom(ctx, "pelanggan", "jenis_bangunan", "jenis_bangunan", "NULL"),
		sumber.ExprKolom(ctx, "pelanggan", "status_bangunan", "status_bangunan", "NULL"),
		sumber.ExprKolom(ctx, "pelanggan", "nominal_pengambilan_pertama", "nominal_pengambilan_pertama", "0"),
		sumber.ExprKolom(ctx, "pelanggan", "estimasi_batas_kredit", "estimasi_batas_kredit", "0"),
		sumber.ExprKolom(ctx, "pelanggan", "created_at", "created_at", "NULL"),
		sumber.ExprKolom(ctx, "pelanggan", "updated_at", "updated_at", "NULL"),
	)

	rows, err := sumber.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	defer rows.Close()

	stmt, err := target.DB().PrepareContext(ctx, `
		INSERT INTO pelanggan
			(id, kode_pelanggan, nama_pelanggan, tgl_registrasi, phone,
			 npwp_nik, nama_pemilik_npwp_nik, alamat_npwp_nik,
			 territory, distrik, alamat_toko, rt_rw,
			 provinsi, kabupaten, kecamatan, kelurahan, kode_pos,
			 channel_outlet, alamat_pengantaran_barang, jenis_bangunan, status_bangunan,
			 nominal_pengambilan_pertama, estimasi_batas_kredit, is_active, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,1,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for rows.Next() {
		var (
			id                                        uint64
			kode, nama, phone, terr, dist, alamatToko sql.NullString
			npwp, pemilik, alamatNPWP, rtrw           sql.NullString
			prov, kab, kec, kel, pos, channel         sql.NullString
			antar, jenisB, statusB                    sql.NullString
			nomS, kreditS                             sql.NullString
			tglReg, created, updated                  sql.NullTime
		)
		if err := rows.Scan(&id, &kode, &nama, &tglReg, &phone,
			&npwp, &pemilik, &alamatNPWP, &terr, &dist, &alamatToko, &rtrw,
			&prov, &kab, &kec, &kel, &pos, &channel, &antar, &jenisB, &statusB,
			&nomS, &kreditS, &created, &updated); err != nil {
			lap.Gagal("pelanggan", 0, err)
			continue
		}

		ch, ok := normalkanChannel(strNS(channel))
		if !ok {
			lap.Gagal("pelanggan", id, fmt.Errorf(
				"channel_outlet tidak terpetakan: %q — koreksi di sumber sebelum cutover", strNS(channel)))
			continue
		}

		npwpVal := placeholderKeNull(strNS(npwp))
		if npwpVal == nil && strings.TrimSpace(strNS(npwp)) != "" {
			lap.Peringatan("pelanggan", id, "npwp_nik placeholder, diisi NULL")
		}

		tgl := tanggalDari(tglReg, tanggalDari(created, created.Time))
		if _, err := stmt.ExecContext(ctx,
			id,
			wajibIsi(strNS(kode), fmt.Sprintf("PLG-%d", id)),
			wajibIsi(strNS(nama), "-"),
			tgl.Format("2006-01-02"),
			wajibIsi(strNS(phone), "-"),
			npwpVal,
			placeholderKeNull(strNS(pemilik)),
			placeholderKeNull(strNS(alamatNPWP)),
			wajibIsi(strNS(terr), "-"),
			wajibIsi(strNS(dist), "-"),
			wajibIsi(strNS(alamatToko), "-"),
			placeholderKeNull(strNS(rtrw)),
			wajibIsi(strNS(prov), "-"),
			wajibIsi(strNS(kab), "-"),
			wajibIsi(strNS(kec), "-"),
			wajibIsi(strNS(kel), "-"),
			placeholderKeNull(strNS(pos)),
			string(ch),
			placeholderKeNull(strNS(antar)),
			placeholderKeNull(strNS(jenisB)),
			placeholderKeNull(strNS(statusB)),
			money.RoundMoney(decNS(nomS)),
			money.RoundMoney(decNS(kreditS)),
			waktuAtauNil(created), waktuAtauNil(updated),
		); err != nil {
			lap.Gagal("pelanggan", id, err)
			continue
		}
		lap.Berhasil("pelanggan")
	}
	if err := rows.Err(); err != nil {
		return err
	}
	resetAutoIncrement(ctx, target, "pelanggan")
	return nil
}

// normalkanChannel memetakan variasi ejaan channel_outlet ke enum (09 §4 tahap 4).
func normalkanChannel(mentah string) (domain.ChannelOutlet, bool) {
	kunci := strings.Join(strings.Fields(strings.ToLower(mentah)), " ")
	peta := map[string]domain.ChannelOutlet{
		"modern trade":             domain.ChannelModernTrade,
		"modern trade independent": domain.ChannelModernTradeIndependent,
		"mt independent":           domain.ChannelModernTradeIndependent,
		"general trade":            domain.ChannelGeneralTrade,
		"general trade kosmetik":   domain.ChannelGeneralTradeKosmetik,
		"gt kosmetik":              domain.ChannelGeneralTradeKosmetik,
		"sub agen":                 domain.ChannelSubAgen,
		"subagen":                  domain.ChannelSubAgen,
	}
	v, ok := peta[kunci]
	return v, ok
}
