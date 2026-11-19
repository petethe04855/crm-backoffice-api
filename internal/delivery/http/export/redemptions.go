package export

import (
	"fmt"
	"strings"
	"time"

	domainmember "crm-core/domain/member"
	domainreward "crm-core/domain/reward"

	"github.com/xuri/excelize/v2"
)

var redemptionStatusTH = map[string]string{
	domainreward.RedemptionPending:   "รอตรวจ",
	domainreward.RedemptionApproved:  "อนุมัติแล้ว",
	domainreward.RedemptionPacking:   "กำลังแพ็ก",
	domainreward.RedemptionShipped:   "จัดส่งแล้ว",
	domainreward.RedemptionDelivered: "ได้รับแล้ว",
	domainreward.RedemptionCancelled: "ยกเลิก",
}

// RedemptionsXLSX renders redemptions. Phone and the street address are
// masked unless unmasked is true (owner only, PDPA); province stays visible
// for reports.
func RedemptionsXLSX(rows []domainreward.RedemptionRow, unmasked bool, loc *time.Location) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	const sheet = "Redemptions"
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		return nil, err
	}
	headers := []any{"เลขที่", "วันที่แลก", "สมาชิก", "ชื่อ", "ของรางวัล", "SKU", "จำนวน", "แต้ม", "สถานะ",
		"ผู้รับ", "เบอร์โทร", "ที่อยู่", "จังหวัด", "รหัสไปรษณีย์", "เลขพัสดุ", "อ้างอิง ERP", "เหตุผลยกเลิก"}
	if err := f.SetSheetRow(sheet, "A1", &headers); err != nil {
		return nil, err
	}
	for i, r := range rows {
		phone, address := r.ShipPhone, r.ShipAddress
		if !unmasked {
			phone = domainmember.MaskPhone(phone)
			if address != "" {
				address = "***"
			}
		}
		status := redemptionStatusTH[r.Status]
		if status == "" {
			status = r.Status
		}
		row := []any{
			r.RedemptionNo, r.CreatedAt.In(loc).Format("2006-01-02 15:04"), r.MemberCode,
			strings.TrimSpace(r.FirstName + " " + r.LastName), r.RewardName, r.SKUCode, r.Qty, r.Points, status,
			r.ShipName, phone, address, r.ShipProvince, r.ShipPostcode, r.TrackingNo, r.StockIssueRef, r.CancelReason,
		}
		if err := f.SetSheetRow(sheet, fmt.Sprintf("A%d", i+2), &row); err != nil {
			return nil, err
		}
	}
	if err := f.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		return nil, err
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
