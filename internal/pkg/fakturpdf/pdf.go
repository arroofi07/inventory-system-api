package fakturpdf

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"

	"github.com/go-pdf/fpdf"

	"app/internal/dto"
)

// NamaBerkas menghasilkan nama file PDF yang aman (memuat no_transaksi bila ada).
func NamaBerkas(noTransaksi *string, id uint64) string {
	if noTransaksi != nil {
		n := sanitasiNama(strings.TrimSpace(*noTransaksi))
		if n != "" {
			return "faktur-" + n + ".pdf"
		}
	}
	return fmt.Sprintf("faktur-%d.pdf", id)
}

func sanitasiNama(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func ptrStr(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

func chunkItems(items []dto.FakturItem, perPage int) [][]dto.FakturItem {
	if len(items) == 0 {
		return [][]dto.FakturItem{{}}
	}
	if perPage < 1 {
		perPage = len(items)
	}
	out := make([][]dto.FakturItem, 0, (len(items)+perPage-1)/perPage)
	for i := 0; i < len(items); i += perPage {
		j := i + perPage
		if j > len(items) {
			j = len(items)
		}
		out = append(out, items[i:j])
	}
	return out
}

// Bangun merender JSON faktur yang sama ke PDF A4 (SD-11).
func Bangun(data *dto.FakturResponse, itemsPerPage int) ([]byte, error) {
	if data == nil {
		return nil, fmt.Errorf("data faktur kosong")
	}
	perPage := itemsPerPage
	if data.Layout != "paginated" {
		perPage = len(data.Items)
		if perPage < 1 {
			perPage = 1
		}
	} else if perPage < 1 {
		perPage = 30
	}
	halaman := chunkItems(data.Items, perPage)

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetCompression(false)
	pdf.SetMargins(12, 12, 12)
	pdf.SetAutoPageBreak(true, 16)
	tr := pdf.UnicodeTranslatorFromDescriptor("")

	cell := func(w, h float64, txt, align, border string, ln int) {
		pdf.CellFormat(w, h, tr(txt), border, ln, align, false, 0, "")
	}

	for i, items := range halaman {
		pdf.AddPage()
		tulisKop(pdf, tr, cell, data, i+1, len(halaman))
		tulisTabel(pdf, cell, items)
		terakhir := i == len(halaman)-1
		if !terakhir {
			sub := subtotal(items)
			pdf.Ln(3)
			pdf.SetFont("Helvetica", "", 9)
			cell(0, 5, "Subtotal halaman: "+sub, "R", "", 1)
			continue
		}
		tulisRingkasan(pdf, cell, data)
		tulisTandaTangan(pdf, cell, data)
	}

	if err := pdf.Error(); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func tulisKop(
	pdf *fpdf.Fpdf,
	tr func(string) string,
	cell func(w, h float64, txt, align, border string, ln int),
	data *dto.FakturResponse,
	halamanKe, totalHalaman int,
) {
	if data.CetakUlang {
		pdf.SetFont("Helvetica", "B", 11)
		pdf.SetFillColor(255, 243, 205)
		pdf.CellFormat(186, 8, tr("CETAK ULANG"), "1", 1, "C", true, 0, "")
		pdf.Ln(2)
	}

	pdf.SetFont("Helvetica", "B", 14)
	nama := strings.TrimSpace(data.Perusahaan.Nama)
	if nama == "" {
		nama = "—"
	}
	cell(186, 7, nama, "L", "", 1)
	pdf.SetFont("Helvetica", "", 8)
	if a := strings.TrimSpace(data.Perusahaan.Alamat); a != "" {
		cell(186, 4, a, "L", "", 1)
	}
	meta := ""
	if t := strings.TrimSpace(data.Perusahaan.Telepon); t != "" {
		meta = "Telp. " + t
	}
	if n := strings.TrimSpace(data.Perusahaan.NPWP); n != "" {
		if meta != "" {
			meta += "  "
		}
		meta += "NPWP " + n
	}
	if meta != "" {
		cell(186, 4, meta, "L", "", 1)
	}
	pdf.Ln(2)

	no := "—"
	if data.NoTransaksi != nil && strings.TrimSpace(*data.NoTransaksi) != "" {
		no = strings.TrimSpace(*data.NoTransaksi)
	}
	pdf.SetFont("Helvetica", "B", 11)
	cell(100, 5, "FAKTUR PENJUALAN", "L", "", 0)
	pdf.SetFont("Helvetica", "", 9)
	cell(86, 5, data.Pelanggan.NamaPelanggan, "R", "", 1)
	pdf.SetFont("Helvetica", "", 9)
	cell(100, 4, "No. "+no, "L", "", 0)
	cell(86, 4, data.Pelanggan.KodePelanggan+" · "+data.Pelanggan.ChannelOutlet, "R", "", 1)
	cell(100, 4, "Tanggal "+data.Tanggal, "L", "", 0)
	alamat := strings.TrimSpace(data.Pelanggan.Alamat)
	if alamat == "" {
		alamat = "—"
	}
	cell(86, 4, alamat, "R", "", 1)
	if totalHalaman > 1 {
		cell(186, 4, fmt.Sprintf("Halaman %d / %d", halamanKe, totalHalaman), "R", "", 1)
	}
	pdf.Ln(3)
}

func tulisTabel(pdf *fpdf.Fpdf, cell func(w, h float64, txt, align, border string, ln int), items []dto.FakturItem) {
	pdf.SetFont("Helvetica", "B", 8)
	cell(10, 6, "#", "L", "B", 0)
	cell(70, 6, "Item", "L", "B", 0)
	cell(28, 6, "Batch", "L", "B", 0)
	cell(22, 6, "Qty", "R", "B", 0)
	cell(28, 6, "Harga", "R", "B", 0)
	cell(28, 6, "Total", "R", "B", 1)
	pdf.SetFont("Helvetica", "", 8)
	for _, it := range items {
		nama := it.NamaItem
		if it.IsBonus {
			nama += " (bonus)"
		}
		cell(10, 5, fmt.Sprintf("%d", it.Urutan), "L", "", 0)
		cell(70, 5, nama, "L", "", 0)
		batch := ptrStr(it.NoBatch)
		if batch == "" {
			batch = "—"
		}
		if e := ptrStr(it.Exp); e != "" {
			batch += " / " + e
		}
		cell(28, 5, batch, "L", "", 0)
		cell(22, 5, fmt.Sprintf("%d %s", it.Qty, it.Satuan), "R", "", 0)
		cell(28, 5, it.Harga, "R", "", 0)
		cell(28, 5, it.TotalFinalBaris, "R", "", 1)
		if it.KodeItem != "" {
			pdf.SetFont("Helvetica", "", 7)
			cell(10, 3.5, "", "L", "", 0)
			cell(176, 3.5, it.KodeItem, "L", "", 1)
			pdf.SetFont("Helvetica", "", 8)
		}
	}
}

func subtotal(items []dto.FakturItem) string {
	// Tampilkan jumlah string apa adanya bila satu baris; selain itu jumlahkan numerik sederhana.
	if len(items) == 1 {
		return items[0].TotalFinalBaris
	}
	var sum float64
	ok := true
	for _, it := range items {
		var v float64
		if _, err := fmt.Sscanf(it.TotalFinalBaris, "%f", &v); err != nil {
			ok = false
			break
		}
		sum += v
	}
	if !ok {
		return ""
	}
	return fmt.Sprintf("%.2f", sum)
}

func tulisRingkasan(
	pdf *fpdf.Fpdf,
	cell func(w, h float64, txt, align, border string, ln int),
	data *dto.FakturResponse,
) {
	pdf.Ln(4)
	y := pdf.GetY()
	pdf.SetFont("Helvetica", "", 8)
	cell(186, 4, "Terbilang: "+data.Ringkasan.Terbilang, "L", "", 1)
	if data.Sales != nil && strings.TrimSpace(data.Sales.Name) != "" {
		cell(100, 4, "Sales: "+data.Sales.Name, "L", "", 1)
	}
	kiriBawah := pdf.GetY()

	pdf.SetXY(118, y)
	pdf.SetFont("Helvetica", "", 9)
	cell(40, 5, "DPP", "L", "", 0)
	cell(40, 5, data.Ringkasan.Total, "R", "", 1)
	pdf.SetX(118)
	cell(40, 5, "PPN "+data.Ringkasan.PPNPersen+"%", "L", "", 0)
	cell(40, 5, data.Ringkasan.PPNNominal, "R", "", 1)
	pdf.SetX(118)
	pdf.SetFont("Helvetica", "B", 10)
	cell(40, 6, "Total", "L", "", 0)
	cell(40, 6, data.Ringkasan.TotalAkhir, "R", "", 1)
	if pdf.GetY() < kiriBawah {
		pdf.SetY(kiriBawah)
	}
}

func tulisTandaTangan(pdf *fpdf.Fpdf, cell func(w, h float64, txt, align, border string, ln int), data *dto.FakturResponse) {
	pdf.Ln(14)
	pdf.SetFont("Helvetica", "", 8)
	cell(62, 4, "Penerima", "C", "", 0)
	cell(62, 4, "Pengirim", "C", "", 0)
	cell(62, 4, "Hormat kami", "C", "", 1)
	pdf.Ln(16)
	cell(62, 4, "(                    )", "C", "", 0)
	cell(62, 4, "(                    )", "C", "", 0)
	nama := "(                    )"
	if data.Approver != nil && strings.TrimSpace(data.Approver.Name) != "" {
		nama = data.Approver.Name
	}
	cell(62, 4, nama, "C", "", 1)
}
