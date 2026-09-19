package csvx

import (
	"bytes"
	"encoding/csv"
	"io"
	"strings"
)

// BOMUTF8 byte order mark agar Excel membaca CSV Indonesia dengan benar.
var BOMUTF8 = []byte{0xEF, 0xBB, 0xBF}

// Tulis menulis baris CSV dengan BOM UTF-8 di awal.
func Tulis(w io.Writer, rows [][]string) error {
	if _, err := w.Write(BOMUTF8); err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	cw.Comma = ','
	for _, row := range rows {
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// Bytes menghasilkan CSV BOM UTF-8 di memori (cukup untuk stub/master kecil).
func Bytes(rows [][]string) ([]byte, error) {
	var buf bytes.Buffer
	if err := Tulis(&buf, rows); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// BacaRecords membaca CSV (dengan atau tanpa BOM), mengembalikan semua baris.
func BacaRecords(data []byte) ([][]string, error) {
	data = bytes.TrimPrefix(data, BOMUTF8)
	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = ','
	r.TrimLeadingSpace = true
	r.FieldsPerRecord = -1
	return r.ReadAll()
}

// IndeksHeader memetakan nama kolom (lowercase trim) → indeks.
func IndeksHeader(header []string) map[string]int {
	out := make(map[string]int, len(header))
	for i, h := range header {
		key := strings.ToLower(strings.TrimSpace(h))
		if key != "" {
			out[key] = i
		}
	}
	return out
}

// AmbilKolom membaca nilai kolom dari baris; kosong bila tidak ada.
func AmbilKolom(row []string, idx map[string]int, name string) string {
	i, ok := idx[strings.ToLower(name)]
	if !ok || i < 0 || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

// LewatiBaris true untuk baris kosong / komentar (#...).
func LewatiBaris(row []string) bool {
	if len(row) == 0 {
		return true
	}
	allEmpty := true
	for _, c := range row {
		if strings.TrimSpace(c) != "" {
			allEmpty = false
			break
		}
	}
	if allEmpty {
		return true
	}
	first := strings.TrimSpace(row[0])
	return strings.HasPrefix(first, "#")
}
