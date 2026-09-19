package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Migrasi users — SF-03. Hash password Laravel dipertahankan.
// Duplikat no_ktp: baris pertama tetap, sisanya NULL (09 §4 tahap 1).
func migrasiUsers(ctx context.Context, sumber *Sumber, target *Target, lap *Laporan) error {
	if skipSumber(sumber, lap, "users") {
		return nil
	}
	if !sumber.PunyaTabel(ctx, "users") {
		return fmt.Errorf("tabel sumber users tidak ada")
	}

	q := fmt.Sprintf(`
		SELECT
			id,
			%s,
			%s,
			%s,
			%s,
			%s,
			%s,
			%s,
			%s,
			%s,
			%s,
			%s,
			%s
		FROM users
		ORDER BY id`,
		sumber.ExprKolom(ctx, "users", "name", "name", "''"),
		sumber.ExprKolom(ctx, "users", "email", "email", "''"),
		sumber.ExprKolom(ctx, "users", "email_verified_at", "email_verified_at", "NULL"),
		sumber.ExprKolom(ctx, "users", "password", "password", "''"),
		sumber.ExprKolom(ctx, "users", "role", "role", "'sales'"),
		sumber.ExprKolom(ctx, "users", "no_hp", "no_hp", "NULL"),
		sumber.ExprKolom(ctx, "users", "no_ktp", "no_ktp", "NULL"),
		sumber.ExprKolom(ctx, "users", "alamat", "alamat", "NULL"),
		sumber.ExprKolom(ctx, "users", "jenis_kelamin", "jenis_kelamin", "NULL"),
		sumber.ExprKolom(ctx, "users", "is_active", "is_active", "1"),
		sumber.ExprKolom(ctx, "users", "created_at", "created_at", "NULL"),
		sumber.ExprKolom(ctx, "users", "updated_at", "updated_at", "NULL"),
	)

	rows, err := sumber.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	defer rows.Close()

	seenKTP := map[string]uint64{}
	stmt, err := target.DB().PrepareContext(ctx, `
		INSERT INTO users
			(id, name, email, email_verified_at, password, role, no_hp, no_ktp,
			 alamat, jenis_kelamin, is_active, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for rows.Next() {
		var (
			id                          uint64
			name, email, password, role string
			emailVer, created, updated  sql.NullTime
			noHP, noKTP, alamat, jk     sql.NullString
			aktif                       sql.NullInt64
		)
		if err := rows.Scan(&id, &name, &email, &emailVer, &password, &role,
			&noHP, &noKTP, &alamat, &jk, &aktif, &created, &updated); err != nil {
			lap.Gagal("users", 0, err)
			continue
		}
		r, ok := roleValid(role)
		if !ok {
			lap.Gagal("users", id, fmt.Errorf("role tidak dikenal: %q", role))
			continue
		}
		ktpVal := trimPtr(strNS(noKTP))
		if s, ok := ktpVal.(string); ok {
			key := strings.TrimSpace(s)
			if prev, ada := seenKTP[key]; ada {
				lap.Peringatan("users", id, fmt.Sprintf(
					"duplikat no_ktp %q (pertama id=%d); diisi NULL", key, prev))
				ktpVal = nil
			} else {
				seenKTP[key] = id
			}
		}

		if _, err := stmt.ExecContext(ctx,
			id, wajibIsi(name, fmt.Sprintf("user-%d", id)), strings.TrimSpace(email),
			waktuAtauNil(emailVer), password, string(r),
			trimPtr(strNS(noHP)), ktpVal, trimPtr(strNS(alamat)),
			jenisKelaminAtauNil(strNS(jk)), boolNS(aktif, true),
			waktuAtauNil(created), waktuAtauNil(updated),
		); err != nil {
			lap.Gagal("users", id, err)
			continue
		}
		lap.Berhasil("users")
	}
	if err := rows.Err(); err != nil {
		return err
	}
	resetAutoIncrement(ctx, target, "users")
	return nil
}
