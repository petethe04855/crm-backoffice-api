package export

import (
	"fmt"

	domainsales "crm-core/domain/sales"

	"github.com/xuri/excelize/v2"
)

// Sales dashboard sections that can be exported, one per tab.
const (
	SalesSummary    = "summary"
	SalesProducts   = "products"
	SalesProvinces  = "provinces"
	SalesCustomers  = "customers"
	SalesTiming     = "timing"
	SalesRepurchase = "repurchase"
)

type sheet struct {
	name    string
	headers []any
	rows    [][]any
}

func pct(v float64) string { return fmt.Sprintf("%.2f%%", v) }

func optPct(v *float64) any {
	if v == nil {
		return ""
	}
	return pct(*v)
}

// SalesDashboardXLSX renders one dashboard section. maskBuyer is applied to
// buyer usernames (pass nil to keep them, owner only).
func SalesDashboardXLSX(r domainsales.Report, section string, maskBuyer func(string) string) ([]byte, error) {
	var sheets []sheet
	switch section {
	case SalesSummary:
		s := sheet{name: "Summary", headers: []any{"ตัวชี้วัด", "ค่า"}}
		s.rows = [][]any{
			{"ช่วงวันที่", r.From + " ถึง " + r.To},
			{"ช่องทาง", r.Channel},
			{"ภูมิภาค", r.Region},
			{"ยอดขาย (บาท)", r.Summary.Sales},
			{"ออเดอร์", r.Summary.Orders},
			{"ผู้ซื้อ", r.Summary.Buyers},
			{"จำนวนชิ้น", r.Summary.Items},
			{"ยอดต่อบิล (AOV)", r.Summary.AOV},
		}
		if r.Previous != nil {
			s.rows = append(s.rows, []any{"ยอดขายช่วงก่อน", r.Previous.Sales}, []any{"ออเดอร์ช่วงก่อน", r.Previous.Orders})
		}
		ch := sheet{name: "Channels", headers: []any{"ช่องทาง", "ออเดอร์", "ยอดขาย", "สัดส่วน"}}
		for _, c := range r.Channels {
			ch.rows = append(ch.rows, []any{c.Label, c.Orders, c.Sales, pct(c.SharePercent)})
		}
		trend := func(name string, pts []domainsales.TrendPoint) sheet {
			t := sheet{name: name, headers: []any{"ช่วง", "เริ่ม", "สิ้นสุด", "Shopee", "TikTok", "รวม", "ออเดอร์ Shopee", "ออเดอร์ TikTok", "ออเดอร์รวม"}}
			for _, p := range pts {
				t.rows = append(t.rows, []any{p.Period, p.StartDate, p.EndDate, p.ShopeeSales, p.TikTokSales, p.TotalSales, p.ShopeeOrders, p.TikTokOrders, p.TotalOrders})
			}
			return t
		}
		sheets = []sheet{s, ch, trend("Weekly", r.Trend.Weekly), trend("Monthly", r.Trend.Monthly)}
	case SalesProducts:
		fl := sheet{name: "Flavors", headers: []any{"รสชาติ", "จำนวนถุง", "ออเดอร์", "ยอดขาย", "สัดส่วน", "เทียบช่วงก่อน"}}
		for _, f := range r.Flavors {
			fl.rows = append(fl.rows, []any{f.Label, f.Units, f.Orders, f.Sales, pct(f.SharePercent), optPct(f.ChangePercent)})
		}
		pr := sheet{name: "Products", headers: []any{"SKU", "สินค้า", "จำนวน", "ออเดอร์", "ยอดขาย", "สัดส่วน"}}
		for _, p := range r.Products {
			pr.rows = append(pr.rows, []any{p.SKU, p.Name, p.Units, p.Orders, p.Sales, pct(p.SharePercent)})
		}
		st := sheet{name: "Sets", headers: []any{"เซ็ต", "ออเดอร์", "สัดส่วน"}}
		for _, s := range r.Sets {
			st.rows = append(st.rows, []any{s.Name, s.Orders, pct(s.SharePercent)})
		}
		ac := sheet{name: "Accessories", headers: []any{"ของเสริม", "จำนวน", "สัดส่วน"}}
		for _, a := range r.Accessories {
			ac.rows = append(ac.rows, []any{a.Label, a.Units, pct(a.SharePercent)})
		}
		sheets = []sheet{fl, pr, st, ac}
	case SalesProvinces:
		pv := sheet{name: "Provinces", headers: []any{"จังหวัด", "ภูมิภาค", "ผู้ซื้อ", "ออเดอร์", "ยอดขาย", "AOV", "สัดส่วน"}}
		for _, p := range r.Provinces {
			pv.rows = append(pv.rows, []any{p.Province, p.Region, p.Buyers, p.Orders, p.Sales, p.AOV, pct(p.SharePercent)})
		}
		rg := sheet{name: "Regions", headers: []any{"ภูมิภาค", "จำนวนจังหวัด", "ออเดอร์", "ยอดขาย", "สัดส่วน"}}
		for _, x := range r.Regions {
			rg.rows = append(rg.rows, []any{x.Region, x.ProvinceCount, x.Orders, x.Sales, pct(x.SharePercent)})
		}
		sheets = []sheet{pv, rg}
	case SalesCustomers:
		cu := sheet{name: "Customers", headers: []any{"แพลตฟอร์ม", "ผู้ซื้อ", "ออเดอร์", "ยอดขาย", "AOV", "จำนวนชิ้น", "ซื้อครั้งแรก", "ซื้อล่าสุด", "จังหวัด", "ความถี่"}}
		for _, c := range r.Customers {
			buyer := c.Buyer
			if maskBuyer != nil {
				buyer = maskBuyer(buyer)
			}
			cu.rows = append(cu.rows, []any{c.Platform, buyer, c.Orders, c.Sales, c.AOV, c.Items, c.FirstOrder, c.LastOrder, c.Province, c.FrequencyBucket})
		}
		fq := sheet{name: "Frequency", headers: []any{"ความถี่", "ผู้ซื้อ", "สัดส่วน"}}
		for _, f := range r.Frequencies {
			fq.rows = append(fq.rows, []any{f.Label, f.Buyers, pct(f.SharePercent)})
		}
		sheets = []sheet{cu, fq}
	case SalesTiming:
		hr := sheet{name: "Hours", headers: []any{"ชั่วโมง", "ออเดอร์", "ยอดขาย"}}
		for _, h := range r.Timing.Hours {
			hr.rows = append(hr.rows, []any{fmt.Sprintf("%02d:00", h.Hour), h.Orders, h.Sales})
		}
		wd := sheet{name: "Weekdays", headers: []any{"วัน", "ออเดอร์", "ยอดขาย"}}
		for _, w := range r.Timing.Weekdays {
			wd.rows = append(wd.rows, []any{w.Label, w.Orders, w.Sales})
		}
		dm := sheet{name: "DayOfMonth", headers: []any{"วันที่", "ออเดอร์", "ยอดขาย"}}
		for _, d := range r.Timing.DaysOfMonth {
			dm.rows = append(dm.rows, []any{d.Day, d.Orders, d.Sales})
		}
		sheets = []sheet{hr, wd, dm}
	case SalesRepurchase:
		rp := sheet{name: "Repurchase", headers: []any{"ช่วงห่างการซื้อซ้ำ", "ผู้ซื้อ", "สัดส่วน"}}
		for _, b := range r.Repurchase.Buckets {
			rp.rows = append(rp.rows, []any{b.Label, b.Buyers, pct(b.SharePercent)})
		}
		rp.rows = append(rp.rows, []any{}, []any{"ผู้ซื้อซ้ำ", r.Repurchase.RepeatBuyers, pct(r.Repurchase.RepeatRatePercent)},
			[]any{"ระยะห่างเฉลี่ย (วัน)", r.Repurchase.AverageGapDays})
		sheets = []sheet{rp}
	default:
		return nil, fmt.Errorf("unknown sales dashboard section %q", section)
	}
	return writeSheets(sheets)
}

func writeSheets(sheets []sheet) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	for i, s := range sheets {
		if i == 0 {
			if err := f.SetSheetName("Sheet1", s.name); err != nil {
				return nil, err
			}
		} else if _, err := f.NewSheet(s.name); err != nil {
			return nil, err
		}
		if err := f.SetSheetRow(s.name, "A1", &s.headers); err != nil {
			return nil, err
		}
		for j, row := range s.rows {
			row := row
			if err := f.SetSheetRow(s.name, fmt.Sprintf("A%d", j+2), &row); err != nil {
				return nil, err
			}
		}
		if err := f.SetPanes(s.name, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
			return nil, err
		}
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
