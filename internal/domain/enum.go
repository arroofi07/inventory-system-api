package domain

import "strings"

type Role string

const (
	RoleSuperAdmin Role = "super_admin"
	RoleAdmin      Role = "admin"
	RoleAfiliasi   Role = "afiliasi"
	RoleSales      Role = "sales"
)

type JenisKelamin string

const (
	JenisKelaminLaki      JenisKelamin = "L"
	JenisKelaminPerempuan JenisKelamin = "P"
)

type ChannelOutlet string

const (
	ChannelModernTrade            ChannelOutlet = "Modern Trade"
	ChannelModernTradeIndependent ChannelOutlet = "Modern Trade Independent"
	ChannelGeneralTrade           ChannelOutlet = "General Trade"
	ChannelGeneralTradeKosmetik   ChannelOutlet = "General Trade Kosmetik"
	ChannelSubAgen                ChannelOutlet = "Sub Agen"
)

// ChannelOutletValid mengembalikan true bila nilai enum channel dikenal.
func ChannelOutletValid(c ChannelOutlet) bool {
	switch c {
	case ChannelModernTrade, ChannelModernTradeIndependent, ChannelGeneralTrade,
		ChannelGeneralTradeKosmetik, ChannelSubAgen:
		return true
	default:
		return false
	}
}

// IsModernTrade menentukan apakah channel memakai harga MT.
func (c ChannelOutlet) IsModernTrade() bool {
	return strings.Contains(strings.ToLower(string(c)), "modern trade")
}

type MarkupType string

const (
	MarkupPercent MarkupType = "percent"
	MarkupValue   MarkupType = "value"
)

type StatusApproval string

const (
	ApprovalPending  StatusApproval = "pending"
	ApprovalApproved StatusApproval = "approved"
	ApprovalRejected StatusApproval = "rejected"
)

type StatusPembayaran string

const (
	PembayaranLunas    StatusPembayaran = "lunas"
	PembayaranHutang   StatusPembayaran = "hutang"
	PembayaranSebagian StatusPembayaran = "sebagian"
)

type FulfillmentStatus string

const (
	FulfillmentAwaitingApproval FulfillmentStatus = "awaiting_approval"
	FulfillmentFull             FulfillmentStatus = "full"
	FulfillmentPartial          FulfillmentStatus = "partial"
	FulfillmentBackorder        FulfillmentStatus = "backorder"
	FulfillmentStockShortage    FulfillmentStatus = "stock_shortage"
)

type TipePromo string

const (
	PromoBuyXGetY           TipePromo = "buy_x_get_y"
	PromoBonusQty           TipePromo = "bonus_qty"
	PromoPercentageDiscount TipePromo = "percentage_discount"
	PromoFixedDiscount      TipePromo = "fixed_discount"
)

type MovementType string

const (
	MovementPenerimaan  MovementType = "PENERIMAAN"
	MovementPenjualan   MovementType = "PENJUALAN"
	MovementPembatalan  MovementType = "PEMBATALAN"
	MovementPenyesuaian MovementType = "PENYESUAIAN"
	MovementOpname      MovementType = "OPNAME"
)

type MetodeAlokasi string

const (
	AlokasiFEFO MetodeAlokasi = "FEFO"
	AlokasiFIFO MetodeAlokasi = "FIFO"
)

func (m MetodeAlokasi) Valid() bool {
	switch m {
	case AlokasiFEFO, AlokasiFIFO:
		return true
	default:
		return false
	}
}

type AlertType string

const (
	AlertStokRendah   AlertType = "STOK_RENDAH"
	AlertStokHabis    AlertType = "STOK_HABIS"
	AlertMendekatiExp AlertType = "MENDEKATI_EXP"
	AlertKedaluwarsa  AlertType = "KEDALUWARSA"
)

type AlertSeverity string

const (
	SeverityRendah AlertSeverity = "RENDAH"
	SeveritySedang AlertSeverity = "SEDANG"
	SeverityTinggi AlertSeverity = "TINGGI"
	SeverityKritis AlertSeverity = "KRITIS"
)
