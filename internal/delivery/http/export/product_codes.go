package export

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"time"

	usecaseproductcode "crm-core/usecase/productcode"

	"github.com/xuri/excelize/v2"
)

var productCodeHeaders = []string{"ลำดับ", "รหัส", "ล็อตรหัส", "รหัสสินค้า", "ชื่อสินค้า", "เลขล็อตสินค้า", "แต้มต่อรหัส", "หมดอายุ"}

func productCodeRow(i int, code string, e *usecaseproductcode.Export, loc *time.Location) []string {
	b := e.Batch
	expires := ""
	if b.CodeExpiresAt != nil {
		expires = b.CodeExpiresAt.In(loc).Format("2006-01-02")
	}
	return []string{
		fmt.Sprint(i + 1), code, b.BatchNo, b.SKUCode, b.SKUNameSnapshot, b.LotNumber, fmt.Sprint(b.PointsPerCode), expires,
	}
}

// ProductCodesCSV renders the print file as UTF-8 CSV with a BOM, so Excel
// opens the Thai text correctly.
func ProductCodesCSV(e *usecaseproductcode.Export, loc *time.Location) ([]byte, error) {
	var buf bytes.Buffer
	buf.Write([]byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM
	w := csv.NewWriter(&buf)
	if err := w.Write(productCodeHeaders); err != nil {
		return nil, err
	}
	for i, code := range e.Codes {
		if err := w.Write(productCodeRow(i, code, e, loc)); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

// ProductCodesXLSX renders the print file as a spreadsheet (codes as text).
func ProductCodesXLSX(e *usecaseproductcode.Export, loc *time.Location) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	const sheet = "Codes"
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		return nil, err
	}
	sw, err := f.NewStreamWriter(sheet)
	if err != nil {
		return nil, err
	}
	header := make([]any, len(productCodeHeaders))
	for i, h := range productCodeHeaders {
		header[i] = h
	}
	if err := sw.SetRow("A1", header); err != nil {
		return nil, err
	}
	for i, code := range e.Codes {
		row := productCodeRow(i, code, e, loc)
		cells := make([]any, len(row))
		for j, v := range row {
			cells[j] = v
		}
		cell, _ := excelize.CoordinatesToCellName(1, i+2)
		if err := sw.SetRow(cell, cells); err != nil {
			return nil, err
		}
	}
	if err := sw.Flush(); err != nil {
		return nil, err
	}
	out, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
