package payload

// Basic — contribuição mínima (recompensa base).
type Basic struct {
	CultureCode  string  `json:"culture_code"   validate:"required,lowercase,min=2,max=30"`
	AreaHa       float64 `json:"area_ha"        validate:"required,gt=0,lte=100000"`
	TotalCostBRL float64 `json:"total_cost_brl" validate:"required,gt=0"`
}

// CostByInput — custo discriminado por insumo, em BRL (total da safra, não por ha).
type CostByInput struct {
	FertilizerBRL float64 `json:"fertilizer_brl" validate:"gte=0"`
	PesticideBRL  float64 `json:"pesticide_brl"  validate:"gte=0"`
	SeedBRL       float64 `json:"seed_brl"       validate:"gte=0"`
	FuelBRL       float64 `json:"fuel_brl"       validate:"gte=0"`
	LaborBRL      float64 `json:"labor_brl"      validate:"gte=0"`
}

// SuppliersByInput — nome do fornecedor por categoria. Nunca sai do enclave: entra só
// no cálculo de "preço médio pago por tipo de insumo na região" (README raiz §6.2).
type SuppliersByInput struct {
	Fertilizer string `json:"fertilizer,omitempty" validate:"max=120"`
	Pesticide  string `json:"pesticide,omitempty"  validate:"max=120"`
	Seed       string `json:"seed,omitempty"       validate:"max=120"`
	Fuel       string `json:"fuel,omitempty"       validate:"max=120"`
}

// Intermediate — dado mais raro e mais útil (recompensa maior).
type Intermediate struct {
	Basic
	CostByInput  CostByInput      `json:"cost_by_input"  validate:"required"`
	YieldSacksHa float64          `json:"yield_sacks_ha" validate:"required,gt=0,lte=500"`
	PlantingDate string           `json:"planting_date"  validate:"required,datetime=2006-01-02"`
	HarvestDate  string           `json:"harvest_date"   validate:"required,datetime=2006-01-02"`
	Suppliers    SuppliersByInput `json:"suppliers"`
}

type Irrigation struct {
	Used   bool   `json:"used"`
	System string `json:"system,omitempty" validate:"omitempty,oneof=center_pivot drip sprinkler furrow other"`
}

type PestEvent struct {
	Name       string `json:"name"       validate:"required,max=80"`
	Management string `json:"management" validate:"required,max=200"`
}

type ClimateLoss struct {
	Event   string  `json:"event"    validate:"required,oneof=drought hail frost flood heat_wave other"`
	AreaPct float64 `json:"area_pct" validate:"gte=0,lte=100"`
}

type Mechanization struct {
	Type      string   `json:"type"      validate:"required,oneof=own outsourced mixed"`
	Machinery []string `json:"machinery" validate:"dive,max=80"`
}

// Advanced — dado denso, alto valor para seguradora e banco (recompensa mais alta).
type Advanced struct {
	Intermediate
	SoilType        string        `json:"soil_type"        validate:"required,max=60"`
	RotationHistory []string      `json:"rotation_history" validate:"required,min=1,max=10,dive,lowercase,min=2,max=30"`
	Irrigation      Irrigation    `json:"irrigation"`
	PestEvents      []PestEvent   `json:"pest_events"      validate:"dive"`
	ClimateLosses   []ClimateLoss `json:"climate_losses"   validate:"dive"`
	Mechanization   Mechanization `json:"mechanization"    validate:"required"`
}
