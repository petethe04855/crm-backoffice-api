// Package export renders spreadsheet downloads (presentation only; the rows,
// including PDPA masking, come from crm-core usecases).
package export

import (
	"fmt"

	usecasemember "crm-core/usecase/member"

	"github.com/xuri/excelize/v2"
)

// MembersXLSX renders the member export. Headers are Thai, matching the UI.
func MembersXLSX(rows []usecasemember.ExportRow) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	const sheet = "Members"
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		return nil, err
	}
	headers := []any{"รหัสสมาชิก", "ชื่อ", "นามสกุล", "อีเมล", "เบอร์โทร", "ระดับ", "สถานะ", "แต้มคงเหลือ", "ยอดซื้อสะสม (บาท)", "วันที่สมัคร"}
	if err := f.SetSheetRow(sheet, "A1", &headers); err != nil {
		return nil, err
	}
	for i, r := range rows {
		cell := fmt.Sprintf("A%d", i+2)
		row := []any{
			r.MemberCode, r.FirstName, r.LastName, r.Email, r.Phone, r.TierName, r.Status,
			r.PointBalance, r.LifetimeSpend, r.RegisteredAt.Format("2006-01-02 15:04"),
		}
		if err := f.SetSheetRow(sheet, cell, &row); err != nil {
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
