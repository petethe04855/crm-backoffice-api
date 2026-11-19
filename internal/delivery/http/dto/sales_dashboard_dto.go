package dto

import (
	"math"

	domainsales "crm-core/domain/sales"
)

// SalesRangeResponse is the effective filter of a dashboard request.
type SalesRangeResponse struct {
	From string `json:"from"`
	To   string `json:"to"`
	Days int    `json:"days"`
}

// SalesSummaryResponse is one KPI block, with the change against the previous
// period of equal length. Change fields are null when the previous value is 0.
type SalesSummaryResponse struct {
	Sales               float64  `json:"sales"`
	Orders              int64    `json:"orders"`
	Buyers              int64    `json:"buyers"`
	Items               int64    `json:"items"`
	AOV                 float64  `json:"aov"`
	SalesChangePercent  *float64 `json:"salesChangePercent"`
	OrdersChangePercent *float64 `json:"ordersChangePercent"`
	BuyersChangePercent *float64 `json:"buyersChangePercent"`
	AOVChangePercent    *float64 `json:"aovChangePercent"`
}

func ToSalesSummaryResponse(cur domainsales.Summary, prev *domainsales.Summary) SalesSummaryResponse {
	out := SalesSummaryResponse{
		Sales:  cur.Sales,
		Orders: cur.Orders,
		Buyers: cur.Buyers,
		Items:  cur.Items,
		AOV:    cur.AOV,
	}
	if prev == nil {
		return out
	}
	out.SalesChangePercent = pctChange(cur.Sales, prev.Sales)
	out.OrdersChangePercent = pctChange(float64(cur.Orders), float64(prev.Orders))
	out.BuyersChangePercent = pctChange(float64(cur.Buyers), float64(prev.Buyers))
	out.AOVChangePercent = pctChange(cur.AOV, prev.AOV)
	return out
}

func pctChange(cur, prev float64) *float64 {
	if prev == 0 {
		return nil
	}
	v := math.Round((cur-prev)/prev*10000) / 100
	return &v
}

// SalesChannelResponse is one channel's share of the period.
type SalesChannelResponse struct {
	Channel      string  `json:"channel"`
	Label        string  `json:"label"`
	Orders       int64   `json:"orders"`
	Sales        float64 `json:"sales"`
	SharePercent float64 `json:"sharePercent"`
}

// SalesTrendPointResponse is one week or month.
type SalesTrendPointResponse struct {
	Period       string  `json:"period"`
	StartDate    string  `json:"startDate"`
	EndDate      string  `json:"endDate"`
	ShopeeSales  float64 `json:"shopeeSales"`
	TikTokSales  float64 `json:"tiktokSales"`
	TotalSales   float64 `json:"totalSales"`
	ShopeeOrders int64   `json:"shopeeOrders"`
	TikTokOrders int64   `json:"tiktokOrders"`
	TotalOrders  int64   `json:"totalOrders"`
}

// SalesTrendResponse holds both series.
type SalesTrendResponse struct {
	Weekly  []SalesTrendPointResponse `json:"weekly"`
	Monthly []SalesTrendPointResponse `json:"monthly"`
}

// SalesMonthResponse is one month of a flavor series.
type SalesMonthResponse struct {
	Month        string  `json:"month"`
	Units        int64   `json:"units"`
	Orders       int64   `json:"orders"`
	Sales        float64 `json:"sales"`
	SharePercent float64 `json:"sharePercent"`
}

// SalesFlavorResponse is one flavor row.
type SalesFlavorResponse struct {
	Key           string               `json:"key"`
	Label         string               `json:"label"`
	Units         int64                `json:"units"`
	Orders        int64                `json:"orders"`
	Sales         float64              `json:"sales"`
	SharePercent  float64              `json:"sharePercent"`
	ChangePercent *float64             `json:"changePercent"`
	Months        []SalesMonthResponse `json:"months"`
}

// SalesProductResponse is one SKU row.
type SalesProductResponse struct {
	SKU          string  `json:"sku"`
	Name         string  `json:"name"`
	Units        int64   `json:"units"`
	Orders       int64   `json:"orders"`
	Sales        float64 `json:"sales"`
	SharePercent float64 `json:"sharePercent"`
}

// SalesSetResponse is one set row.
type SalesSetResponse struct {
	Name         string  `json:"name"`
	Orders       int64   `json:"orders"`
	SharePercent float64 `json:"sharePercent"`
}

// SalesAccessoryResponse is one add-on row.
type SalesAccessoryResponse struct {
	Key          string  `json:"key"`
	Label        string  `json:"label"`
	Units        int64   `json:"units"`
	SharePercent float64 `json:"sharePercent"`
}

// SalesProvinceResponse is one province row.
type SalesProvinceResponse struct {
	Province     string  `json:"province"`
	Region       string  `json:"region"`
	Buyers       int64   `json:"buyers"`
	Orders       int64   `json:"orders"`
	Sales        float64 `json:"sales"`
	AOV          float64 `json:"aov"`
	SharePercent float64 `json:"sharePercent"`
}

// SalesRegionResponse is the region roll-up.
type SalesRegionResponse struct {
	Region        string  `json:"region"`
	ProvinceCount int     `json:"provinceCount"`
	Orders        int64   `json:"orders"`
	Sales         float64 `json:"sales"`
	SharePercent  float64 `json:"sharePercent"`
}

// SalesCustomerResponse is one buyer row. Buyer is masked unless the caller may
// see personal data.
type SalesCustomerResponse struct {
	Platform        string  `json:"platform"`
	Buyer           string  `json:"buyer"`
	Orders          int64   `json:"orders"`
	Sales           float64 `json:"sales"`
	AOV             float64 `json:"aov"`
	Items           int64   `json:"items"`
	FirstOrder      string  `json:"firstOrder"`
	LastOrder       string  `json:"lastOrder"`
	Province        string  `json:"province"`
	FrequencyBucket string  `json:"frequencyBucket"`
}

// SalesFrequencyResponse is one bucket of the purchase-frequency chart.
type SalesFrequencyResponse struct {
	Label        string  `json:"label"`
	Buyers       int64   `json:"buyers"`
	SharePercent float64 `json:"sharePercent"`
}

// SalesHourResponse is one hour of the day.
type SalesHourResponse struct {
	Hour   int     `json:"hour"`
	Orders int64   `json:"orders"`
	Sales  float64 `json:"sales"`
}

// SalesWeekdayResponse is one day of the week (1 = Monday).
type SalesWeekdayResponse struct {
	Day    int     `json:"day"`
	Label  string  `json:"label"`
	Orders int64   `json:"orders"`
	Sales  float64 `json:"sales"`
}

// SalesDayResponse is one calendar day of the month.
type SalesDayResponse struct {
	Day    int     `json:"day"`
	Orders int64   `json:"orders"`
	Sales  float64 `json:"sales"`
}

// SalesTimingResponse groups the time distributions.
type SalesTimingResponse struct {
	Hours       []SalesHourResponse    `json:"hours"`
	Weekdays    []SalesWeekdayResponse `json:"weekdays"`
	DaysOfMonth []SalesDayResponse     `json:"daysOfMonth"`
	PeakHour    int                    `json:"peakHour"`
	PeakWeekday string                 `json:"peakWeekday"`
	PeakDay     int                    `json:"peakDay"`
}

// SalesRepurchaseBucketResponse is one gap bucket.
type SalesRepurchaseBucketResponse struct {
	Label        string  `json:"label"`
	MinDays      int     `json:"minDays"`
	MaxDays      int     `json:"maxDays"`
	Buyers       int64   `json:"buyers"`
	SharePercent float64 `json:"sharePercent"`
}

// SalesRepurchaseResponse summarises repeat purchases.
type SalesRepurchaseResponse struct {
	Buckets           []SalesRepurchaseBucketResponse `json:"buckets"`
	RepeatBuyers      int64                           `json:"repeatBuyers"`
	RepeatRatePercent float64                         `json:"repeatRatePercent"`
	AverageGapDays    float64                         `json:"averageGapDays"`
}

// SalesReportResponse is the whole dashboard payload.
type SalesReportResponse struct {
	Range       SalesRangeResponse       `json:"range"`
	Channel     string                   `json:"channel"`
	Region      string                   `json:"region"`
	Masked      bool                     `json:"masked"`
	Summary     SalesSummaryResponse     `json:"summary"`
	Previous    SalesSummaryResponse     `json:"previous"`
	Channels    []SalesChannelResponse   `json:"channels"`
	Trend       SalesTrendResponse       `json:"trend"`
	Flavors     []SalesFlavorResponse    `json:"flavors"`
	Products    []SalesProductResponse   `json:"products"`
	Sets        []SalesSetResponse       `json:"sets"`
	Accessories []SalesAccessoryResponse `json:"accessories"`
	Provinces   []SalesProvinceResponse  `json:"provinces"`
	Regions     []SalesRegionResponse    `json:"regions"`
	Customers   []SalesCustomerResponse  `json:"customers"`
	Frequencies []SalesFrequencyResponse `json:"frequencies"`
	Timing      SalesTimingResponse      `json:"timing"`
	Repurchase  SalesRepurchaseResponse  `json:"repurchase"`
}

// ToSalesReportResponse maps a computed report.
func ToSalesReportResponse(r domainsales.Report, days int) SalesReportResponse {
	out := SalesReportResponse{
		Range:       SalesRangeResponse{From: r.From, To: r.To, Days: days},
		Channel:     r.Channel,
		Region:      r.Region,
		Masked:      r.Masked,
		Summary:     ToSalesSummaryResponse(r.Summary, r.Previous),
		Previous:    ToSalesSummaryResponse(domainsales.Summary{}, nil),
		Channels:    make([]SalesChannelResponse, 0, len(r.Channels)),
		Flavors:     make([]SalesFlavorResponse, 0, len(r.Flavors)),
		Products:    make([]SalesProductResponse, 0, len(r.Products)),
		Sets:        make([]SalesSetResponse, 0, len(r.Sets)),
		Accessories: make([]SalesAccessoryResponse, 0, len(r.Accessories)),
		Provinces:   make([]SalesProvinceResponse, 0, len(r.Provinces)),
		Regions:     make([]SalesRegionResponse, 0, len(r.Regions)),
		Customers:   make([]SalesCustomerResponse, 0, len(r.Customers)),
		Frequencies: make([]SalesFrequencyResponse, 0, len(r.Frequencies)),
	}
	if r.Previous != nil {
		out.Previous = ToSalesSummaryResponse(*r.Previous, nil)
	}

	for _, c := range r.Channels {
		out.Channels = append(out.Channels, SalesChannelResponse{
			Channel: c.Channel, Label: c.Label, Orders: c.Orders,
			Sales: c.Sales, SharePercent: c.SharePercent,
		})
	}
	for _, p := range r.Trend.Weekly {
		out.Trend.Weekly = append(out.Trend.Weekly, toTrendPoint(p))
	}
	for _, p := range r.Trend.Monthly {
		out.Trend.Monthly = append(out.Trend.Monthly, toTrendPoint(p))
	}
	if out.Trend.Weekly == nil {
		out.Trend.Weekly = []SalesTrendPointResponse{}
	}
	if out.Trend.Monthly == nil {
		out.Trend.Monthly = []SalesTrendPointResponse{}
	}
	for _, f := range r.Flavors {
		row := SalesFlavorResponse{
			Key: f.Key, Label: f.Label, Units: f.Units, Orders: f.Orders,
			Sales: f.Sales, SharePercent: f.SharePercent, ChangePercent: f.ChangePercent,
			Months: make([]SalesMonthResponse, 0, len(f.Months)),
		}
		for _, m := range f.Months {
			row.Months = append(row.Months, SalesMonthResponse{
				Month: m.Month, Units: m.Units, Orders: m.Orders,
				Sales: m.Sales, SharePercent: m.SharePercent,
			})
		}
		out.Flavors = append(out.Flavors, row)
	}
	for _, p := range r.Products {
		out.Products = append(out.Products, SalesProductResponse{
			SKU: p.SKU, Name: p.Name, Units: p.Units, Orders: p.Orders,
			Sales: p.Sales, SharePercent: p.SharePercent,
		})
	}
	for _, s := range r.Sets {
		out.Sets = append(out.Sets, SalesSetResponse{Name: s.Name, Orders: s.Orders, SharePercent: s.SharePercent})
	}
	for _, a := range r.Accessories {
		out.Accessories = append(out.Accessories, SalesAccessoryResponse{
			Key: a.Key, Label: a.Label, Units: a.Units, SharePercent: a.SharePercent,
		})
	}
	for _, p := range r.Provinces {
		out.Provinces = append(out.Provinces, SalesProvinceResponse{
			Province: p.Province, Region: p.Region, Buyers: p.Buyers, Orders: p.Orders,
			Sales: p.Sales, AOV: p.AOV, SharePercent: p.SharePercent,
		})
	}
	for _, reg := range r.Regions {
		out.Regions = append(out.Regions, SalesRegionResponse{
			Region: reg.Region, ProvinceCount: reg.ProvinceCount, Orders: reg.Orders,
			Sales: reg.Sales, SharePercent: reg.SharePercent,
		})
	}
	for _, c := range r.Customers {
		out.Customers = append(out.Customers, SalesCustomerResponse{
			Platform: c.Platform, Buyer: c.Buyer, Orders: c.Orders, Sales: c.Sales,
			AOV: c.AOV, Items: c.Items, FirstOrder: c.FirstOrder, LastOrder: c.LastOrder,
			Province: c.Province, FrequencyBucket: c.FrequencyBucket,
		})
	}
	for _, f := range r.Frequencies {
		out.Frequencies = append(out.Frequencies, SalesFrequencyResponse{
			Label: f.Label, Buyers: f.Buyers, SharePercent: f.SharePercent,
		})
	}
	out.Timing = toTimingResponse(r.Timing)
	out.Repurchase = toRepurchaseResponse(r.Repurchase)
	return out
}

func toTrendPoint(p domainsales.TrendPoint) SalesTrendPointResponse {
	return SalesTrendPointResponse{
		Period: p.Period, StartDate: p.StartDate, EndDate: p.EndDate,
		ShopeeSales: p.ShopeeSales, TikTokSales: p.TikTokSales, TotalSales: p.TotalSales,
		ShopeeOrders: p.ShopeeOrders, TikTokOrders: p.TikTokOrders, TotalOrders: p.TotalOrders,
	}
}

func toTimingResponse(t domainsales.Timing) SalesTimingResponse {
	out := SalesTimingResponse{
		Hours:       make([]SalesHourResponse, 0, len(t.Hours)),
		Weekdays:    make([]SalesWeekdayResponse, 0, len(t.Weekdays)),
		DaysOfMonth: make([]SalesDayResponse, 0, len(t.DaysOfMonth)),
		PeakHour:    t.PeakHour,
		PeakWeekday: t.PeakWeekday,
		PeakDay:     t.PeakDay,
	}
	for _, h := range t.Hours {
		out.Hours = append(out.Hours, SalesHourResponse{Hour: h.Hour, Orders: h.Orders, Sales: h.Sales})
	}
	for _, w := range t.Weekdays {
		out.Weekdays = append(out.Weekdays, SalesWeekdayResponse{Day: w.Day, Label: w.Label, Orders: w.Orders, Sales: w.Sales})
	}
	for _, d := range t.DaysOfMonth {
		out.DaysOfMonth = append(out.DaysOfMonth, SalesDayResponse{Day: d.Day, Orders: d.Orders, Sales: d.Sales})
	}
	return out
}

func toRepurchaseResponse(r domainsales.Repurchase) SalesRepurchaseResponse {
	out := SalesRepurchaseResponse{
		Buckets:           make([]SalesRepurchaseBucketResponse, 0, len(r.Buckets)),
		RepeatBuyers:      r.RepeatBuyers,
		RepeatRatePercent: r.RepeatRatePercent,
		AverageGapDays:    r.AverageGapDays,
	}
	for _, b := range r.Buckets {
		out.Buckets = append(out.Buckets, SalesRepurchaseBucketResponse{
			Label: b.Label, MinDays: b.MinDays, MaxDays: b.MaxDays,
			Buyers: b.Buyers, SharePercent: b.SharePercent,
		})
	}
	return out
}

// SalesCustomerOrderResponse is one order in a customer drill-down.
type SalesCustomerOrderResponse struct {
	Platform string  `json:"platform"`
	OrderNo  string  `json:"orderNo"`
	Date     string  `json:"date"`
	Amount   float64 `json:"amount"`
	Channel  string  `json:"channel"`
	Province string  `json:"province"`
	Items    int64   `json:"items"`
}

// SalesCustomerDetailResponse is the drill-down of one buyer.
type SalesCustomerDetailResponse struct {
	Customer SalesCustomerResponse        `json:"customer"`
	Orders   []SalesCustomerOrderResponse `json:"orders"`
}
