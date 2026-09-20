package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"duluin_invoice/utils"
)

// Unit is a company's configurable unit-of-measure master data (e.g. Pcs,
// Box, Kg) — used by Delivery Note / Goods Receipt line items. Shaped like
// Tax/Account (CompanyID, IsSystem, IsActive) but simpler: no accounts, no
// components, just a name.
type Unit struct {
	ID        string        `gorm:"type:uuid;primaryKey"                      json:"id"`
	CompanyID string        `gorm:"type:uuid;not null;index"                  json:"company_id"`
	Name      string        `gorm:"type:varchar(50);not null"                json:"name"`
	Symbol    string        `gorm:"type:varchar(30)"                         json:"symbol,omitempty"`
	IsSystem  utils.BoolInt `gorm:"type:smallint;not null;default:0"          json:"is_system"` // seeded default — name locked
	IsActive  utils.BoolInt `gorm:"type:smallint;not null;default:1"          json:"is_active"`

	CreatedAt time.Time      `gorm:"autoCreateTime"   json:"created_at"`
	CreatedBy string         `gorm:"type:varchar(64)" json:"created_by,omitempty"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"   json:"updated_at"`
	UpdatedBy string         `gorm:"type:varchar(64)" json:"updated_by,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index"            json:"-"`
}

func (Unit) TableName() string { return "units" }

func (u *Unit) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	return nil
}

// UnitSeed is one row of the default unit-of-measure template.
type UnitSeed struct {
	Name   string
	Symbol string
}

// DefaultUnits — the standard unit-of-measure catalog (name + symbol),
// replacing the earlier 12-item list previously hardcoded in the
// invoicing-fe SimpleLineItemsEditor.tsx UNIT_OPTIONS constant. Duplicate
// names differing only by case (e.g. "Botol" and "BOTOL", "Lembar"/"LBR"
// and "LBR"/"LBR") are kept as separate rows exactly as provided — they
// come from an existing external unit master, not a fresh design.
func DefaultUnits() []UnitSeed {
	return []UnitSeed{
		{"KG", "KG"}, {"Karton", "KTN"}, {"Renteng", "RTG"}, {"Can", "Can"}, {"Piece", "pc"},
		{"Lusin", "LUSIN"}, {"KTK", "KTK"}, {"Each", "EA"}, {"Square Meter", "m2"}, {"PK", "PK"},
		{"KPG", "KPG"}, {"BTL", "BTL"}, {"LMBR", "LMBR"}, {"TABUNG", "TABUNG"}, {"Suppositorium", "SUPP"},
		{"Botol", "BTL"}, {"Pak", "PAK"}, {"Tube", "TB"}, {"Flask", "FLS"}, {"Flabot", "FLAB"},
		{"BUKU", "BUKU"}, {"Papan", "Papan"}, {"Pax", "Pax"}, {"BTE", "BTE"}, {"Lembar", "LBR"},
		{"Roll", "Roll"}, {"KARUNG", "KARUNG"}, {"pcs", "pcs"}, {"Mile", "mi"}, {"LTR", "LTR"},
		{"Meter", "m"}, {"M3", "M3"}, {"MTR", "MTR"}, {"bulan", "bulan"}, {"Trip", "TRIP"},
		{"Percent", "%"}, {"Cup", "CUP"}, {"Pouch", "POUCH"}, {"Inch", "in"}, {"Ekor", "EKOR"},
		{"Ounce", "oz"}, {"Kilogram", "kg"}, {"Potong", "PTG"}, {"Square Yard", "sq yd"}, {"Keping", "Keping"},
		{"Centimeter", "cm"}, {"GL", "GL"}, {"Pail", "Pail"}, {"Milliliter", "ml"}, {"galon", "galon"},
		{"Batang", "Btg"}, {"PACK", "PACK"}, {"Gallon", "gal"}, {"Document", "DOC"}, {"Dus", "DUS"},
		{"Ritase", "Rit"}, {"TBG", "TBG"}, {"VH", "VH"}, {"Koli", "koli"}, {"Sak", "sak"},
		{"Kodi", "kodi"}, {"Kata", "kata"}, {"KEPING", "KEPING"}, {"GLN", "GLN"}, {"Buah", "Buah"},
		{"BTNG", "BTNG"}, {"PAC", "PAC"}, {"Square Mile", "sq mi"}, {"Drum", "Drum"}, {"Orang", "Orang"},
		{"Jerigen", "JRG"}, {"Sachet", "SCH"}, {"Hectare", "ha"}, {"Square Millimeter", "mm2"}, {"Gram", "g"},
		{"KLG", "KLG"}, {"BTG", "BTG"}, {"Vial", "VL"}, {"Ampule", "AMPULE"}, {"Pasang", "PASANG"},
		{"Strip", "STRIP"}, {"Yard", "yd"}, {"KP", "KP"}, {"Square Inch", "sq in"}, {"Ret", "RET"},
		{"BH", "BH"}, {"Square Foot", "sq ft"}, {"Bal", "BAL"}, {"Kotak", "Kotak"}, {"Karung", "Karung"},
		{"B", "B"}, {"PSNG", "PSNG"}, {"Foot", "ft"}, {"Bungkus", "Bngkus"}, {"BT", "BT"},
		{"Pint", "pt"}, {"LBR", "LBR"}, {"Millimeter", "mm"}, {"Set", "set"}, {"Week", "WEEK"},
		{"Month", "MONTH"}, {"Tablet", "TABLET"}, {"Botol", "BOTOL"}, {"Box", "BOX"}, {"Mobil", "Mobil"},
		{"Kilometer", "km"}, {"Titik", "Titik"}, {"Unit", "Unit"}, {"Lonjor", "Lonjor"}, {"Paket", "Paket"},
		{"Metric Ton", "t"}, {"Quart", "qt"}, {"GALON", "GALON"}, {"Toples", "Tpl"}, {"Rim", "Rim"},
		{"Shift", "SHIFT"}, {"Rit", "Rit"}, {"PC", "PC"}, {"Square Kilometer", "km2"}, {"Kilobit Per second", "Kbps"},
		{"Megabit Per Second", "Mbps"}, {"KOTAK", "KOTAK"}, {"Gross", "Gross"}, {"PSG", "PSG"}, {"Pound", "lb"},
		{"Kali", "X"}, {"Lot", "LOT"}, {"License", "LCS"}, {"User", "USER"}, {"Year", "YEAR"},
		{"Centiliter", "cl"}, {"KRG", "KRG"}, {"Hour", "hr"}, {"Ikat", "Ikat"}, {"Day", "day"},
		{"Hari", "hari"}, {"menit", "menit"}, {"BKS", "BKS"}, {"Cubic Meter", "m3"}, {"Tahun", "TAHUN"},
		{"Square Centimeter", "cm2"}, {"Butir", "Butir"}, {"GULUNG", "GULUNG"}, {"Tabung", "Tabung"}, {"DRIGEN", "DRIGEN"},
		{"KALENG", "KALENG"}, {"Liter", "l"}, {"Capsul", "CAP"}, {"Blok", "BLOK"}, {"Sisir", "Sisir"},
	}
}
