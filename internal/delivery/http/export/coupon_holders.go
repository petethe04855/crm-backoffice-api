package export

import (
	"fmt"
	"strings"
	"time"

	domaincoupon "crm-core/domain/coupon"
	domainmember "crm-core/domain/member"

	"github.com/xuri/excelize/v2"
)

var holderStatusTH = map[string]string{
	domaincoupon.HolderAvailable: "ใช้ได้",
	domaincoupon.HolderUsed:      "ใช้แล้ว",
	domaincoupon.HolderExpired:   "หมดอายุ",
	domaincoupon.HolderCancelled: "ยกเลิก",
}

// CouponHoldersXLSX renders the holders of one coupon. Email and phone are
// masked unless unmasked is true (owner only, PDPA).
func CouponHoldersXLSX(rows []domaincoupon.Holder, unmasked bool, loc *time.Location) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	const sheet = "Holders"
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		return nil, err
	}
	headers := []any{"รหัสคูปอง", "รหัสสมาชิก", "ชื่อ-นามสกุล", "อีเมล", "เบอร์โทร", "สถานะ", "แต้มที่ใช้", "วันที่แลก", "หมดอายุ", "วันที่ใช้", "เลขบิล", "เหตุผลยกเลิก"}
	if err := f.SetSheetRow(sheet, "A1", &headers); err != nil {
		return nil, err
	}
	stamp := func(t *time.Time) string {
		if t == nil {
			return ""
		}
		return t.In(loc).Format("2006-01-02 15:04")
	}
	for i, r := range rows {
		email, phone := r.Email, ""
		if r.Phone != nil {
			phone = *r.Phone
		}
		if !unmasked {
			email, phone = domainmember.MaskEmail(email), domainmember.MaskPhone(phone)
		}
		status := holderStatusTH[r.Status]
		if status == "" {
			status = r.Status
		}
		claimed, expires := r.ClaimedAt, r.ExpiresAt
		row := []any{
			r.Code, r.MemberCode, strings.TrimSpace(r.FirstName + " " + r.LastName), email, phone, status,
			r.PointsSpent, stamp(&claimed), stamp(&expires), stamp(r.UsedAt), r.UsedRef, r.CancelReason,
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
