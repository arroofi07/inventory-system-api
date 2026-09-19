package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	loadDotEnv()

	var (
		skipTruncate    = flag.Bool("skip-truncate", false, "jangan TRUNCATE target sebelum tahap")
		verifyOnly      = flag.Bool("verify-only", false, "hanya jalankan verifikasi paritas (SF-06)")
		idempotentCheck = flag.Bool("idempotent-check", false, "jalankan ETL 2× dan bandingkan checksum")
		frameworkOnly   = flag.Bool("framework-only", false, "uji kerangka tanpa sumber Laravel (stub tahap + checksum target)")
		reportDirFlag   = flag.String("report-dir", "", "direktori artefak laporan (default ETL_REPORT_DIR)")
		dryRun          = flag.Bool("dry-run", false, "cetak rencana tanpa menulis")
		auditSumberFlag = flag.Bool("audit-sumber", false, "laporkan duplikat/channel/I7/I8 di sumber (read-only, SF-02)")
		bersihkanSumber = flag.Bool("bersihkan-sumber", false, "terapkan koreksi SF-02 pada salinan sumber (butuh ETL_SUMBER_BOLEH_TULIS=true)")
	)
	flag.Parse()

	modeSumber := *auditSumberFlag || *bersihkanSumber
	allowMissing := *dryRun || *frameworkOnly || modeSumber
	cfg, err := LoadETLConfig(allowMissing)
	if err != nil {
		fail("config: %v", err)
	}
	if *reportDirFlag != "" {
		cfg.ReportDir = *reportDirFlag
	}

	if modeSumber {
		if err := jalankanModeSumber(cfg, *bersihkanSumber, *dryRun); err != nil {
			fail("%v", err)
		}
		return
	}

	if *dryRun {
		fmt.Println("=== ETL dry-run ===")
		fmt.Printf("Sumber : %s (read-only)\n", labelAtauKosong(cfg.Sumber))
		fmt.Printf("Target : %s\n", cfg.Target.Label())
		fmt.Printf("Report : %s\n", cfg.ReportDir)
		fmt.Println("Tahap  :")
		for i, t := range daftarTahap() {
			fmt.Printf("  %2d. %s\n", i+1, t.Nama())
		}
		fmt.Println("Verifikasi: V1–V9 (SF-06)")
		fmt.Println("SF-02   : -audit-sumber | -bersihkan-sumber (salinan + ETL_SUMBER_BOLEH_TULIS=true)")
		return
	}

	var sumber *Sumber
	if !*frameworkOnly {
		sumber, err = BukaSumber(cfg.Sumber)
		if err != nil {
			fail("%v", err)
		}
		defer sumber.Close()
	}

	target, err := BukaTarget(cfg.Target)
	if err != nil {
		fail("%v", err)
	}
	defer target.Close()

	sumberLabel := "(framework-only)"
	if sumber != nil {
		sumberLabel = sumber.Label()
	}

	if *verifyOnly {
		if sumber == nil {
			fail("--verify-only membutuhkan sumber Laravel (jangan --framework-only)")
		}
		lap := NewLaporan(sumberLabel, target.Label())
		v := &Verifikator{Sumber: sumber, Target: target}
		ok, hasil := v.Jalankan(context.Background())
		lap.SetVerifikasi(hasil, ringkasKesimpulan(hasil))
		lap.Tutup()
		path, werr := tulisLaporan(cfg.ReportDir, lap)
		if werr != nil {
			fail("tulis laporan: %v", werr)
		}
		fmt.Printf("laporan verifikasi: %s\n", path)
		if !ok {
			os.Exit(1)
		}
		return
	}

	if *idempotentCheck {
		if err := jalankanIdempotentCheck(sumber, target, sumberLabel, cfg.ReportDir, !*skipTruncate); err != nil {
			fail("idempotent-check: %v", err)
		}
		fmt.Println("idempotent-check: LULUS (checksum run1 == run2)")
		return
	}

	lap, err := jalankanETL(sumber, target, sumberLabel, !*skipTruncate)
	if err != nil {
		fail("etl: %v", err)
	}
	path, err := tulisLaporan(cfg.ReportDir, lap)
	if err != nil {
		fail("tulis laporan: %v", err)
	}
	fmt.Printf("laporan ETL: %s\nchecksum: %s\n", path, lap.Checksum)
}

func jalankanETL(sumber *Sumber, target *Target, sumberLabel string, doTruncate bool) (*Laporan, error) {
	ctx := context.Background()
	lap := NewLaporan(sumberLabel, target.Label())

	if doTruncate {
		if err := target.TruncateETLTables(ctx); err != nil {
			return nil, fmt.Errorf("truncate: %w", err)
		}
	}

	for _, tahap := range daftarTahap() {
		if err := tahap.Jalankan(ctx, sumber, target, lap); err != nil {
			lap.Gagal(tahap.Nama(), 0, err)
			continue
		}
	}

	if sumber != nil {
		v := &Verifikator{Sumber: sumber, Target: target}
		_, hasil := v.Jalankan(ctx)
		lap.SetVerifikasi(hasil, ringkasKesimpulan(hasil))
	} else {
		lap.SetVerifikasi(nil, "framework-only: verifikasi V1–V9 dilewati (butuh sumber + SF-06)")
	}

	cs, err := HitungChecksumTarget(ctx, target)
	if err != nil {
		return nil, fmt.Errorf("checksum: %w", err)
	}
	lap.SetChecksum(cs)
	lap.Tutup()
	return lap, nil
}

func jalankanIdempotentCheck(sumber *Sumber, target *Target, sumberLabel, reportDir string, doTruncate bool) error {
	if !doTruncate {
		return fmt.Errorf("--idempotent-check membutuhkan truncate (jangan pakai --skip-truncate)")
	}

	lap1, err := jalankanETL(sumber, target, sumberLabel, true)
	if err != nil {
		return fmt.Errorf("run1: %w", err)
	}
	if _, err := tulisLaporan(reportDir, lap1); err != nil {
		return err
	}

	lap2, err := jalankanETL(sumber, target, sumberLabel, true)
	if err != nil {
		return fmt.Errorf("run2: %w", err)
	}
	if _, err := tulisLaporan(reportDir, lap2); err != nil {
		return err
	}

	return BandingkanChecksum(lap1.Checksum, lap2.Checksum)
}

func jalankanModeSumber(cfg ETLConfig, tulis, dryRun bool) error {
	if cfg.Sumber.Host == "" || cfg.Sumber.Name == "" || cfg.Sumber.User == "" {
		return fmt.Errorf("audit/bersih sumber membutuhkan ETL_SUMBER_HOST, ETL_SUMBER_NAME, ETL_SUMBER_USER")
	}
	apply := tulis && !dryRun
	var sumber *Sumber
	var err error
	if apply {
		sumber, err = BukaSumberTulis(cfg.Sumber)
	} else {
		sumber, err = BukaSumber(cfg.Sumber)
	}
	if err != nil {
		return err
	}
	defer sumber.Close()

	lap, err := auditSumber(context.Background(), sumber)
	if err != nil {
		return err
	}
	switch {
	case apply:
		if err := terapkanBersih(context.Background(), sumber, lap); err != nil {
			return err
		}
	case tulis && dryRun:
		lap.Mode = "RENCANA"
	default:
		lap.Mode = "AUDIT"
	}

	path, err := tulisArtefak(cfg.ReportDir, "audit-sumber", lap.Format)
	if err != nil {
		return err
	}
	fmt.Printf("laporan SF-02: %s\n", path)
	if len(lap.ChannelGagal) > 0 {
		fmt.Printf("peringatan: %d channel belum terpetakan (koreksi manual sebelum ETL)\n", len(lap.ChannelGagal))
	}
	return nil
}

func tulisLaporan(dir string, lap *Laporan) (string, error) {
	return tulisArtefak(dir, "etl", lap.Format)
}

func tulisArtefak(dir, prefix string, format func(io.Writer) error) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("%s-%s.txt", prefix, time.Now().Format("20060102-150405.000"))
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := format(f); err != nil {
		return "", err
	}
	_ = format(os.Stdout)
	fmt.Fprintln(os.Stdout)
	return path, nil
}

func labelAtauKosong(c DBConfig) string {
	if c.Host == "" || c.Name == "" {
		return "(belum dikonfigurasi)"
	}
	return c.Label()
}

func loadDotEnv() {
	candidates := []string{".env", "../.env", "../../.env"}
	for _, p := range candidates {
		if err := godotenv.Load(p); err == nil {
			return
		}
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "etl: "+format+"\n", args...)
	os.Exit(1)
}
