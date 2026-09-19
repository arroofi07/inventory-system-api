package domain_test

import (
	"testing"

	"app/internal/domain"
)

func TestRBACMatrix(t *testing.T) {
	if !domain.RoleSuperAdmin.Punya(domain.PermAuditLihat) {
		t.Fatal("super_admin harus punya audit.lihat")
	}
	if !domain.RoleAdmin.Punya(domain.PermApprovalLakukan) {
		t.Fatal("admin harus boleh approve")
	}
	if domain.RoleAfiliasi.Punya(domain.PermLaporanLaba) {
		t.Fatal("afiliasi tidak boleh laporan.laba")
	}
	if domain.RoleSales.Punya(domain.PermTransaksiLihatSemua) {
		t.Fatal("sales tidak boleh lihat semua transaksi")
	}
	if !domain.RoleSales.Punya(domain.PermTransaksiBuat) {
		t.Fatal("sales harus boleh buat transaksi")
	}
	// super_admin adalah superset admin
	for p := range domain.RoleAdmin.Izin() {
		if !domain.RoleSuperAdmin.Punya(p) {
			t.Fatalf("super_admin missing izin admin: %s", p)
		}
	}
}
