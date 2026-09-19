package csvx_test

import (
	"fmt"
	"testing"
	"time"

	"app/internal/pkg/csvx"
)

func TestBytes10kBarisDiBawah10Detik(t *testing.T) {
	rows := make([][]string, 0, 10001)
	rows = append(rows, []string{"id", "nama", "nilai"})
	for i := 0; i < 10000; i++ {
		rows = append(rows, []string{fmt.Sprintf("%d", i), fmt.Sprintf("item-%d", i), "12345.67"})
	}
	start := time.Now()
	b, err := csvx.Bytes(rows)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) < 1000 {
		t.Fatalf("output terlalu kecil: %d", len(b))
	}
	if elapsed > 10*time.Second {
		t.Fatalf("10k baris terlalu lambat: %s", elapsed)
	}
	t.Logf("10k baris dalam %s (%d bytes)", elapsed, len(b))
}
