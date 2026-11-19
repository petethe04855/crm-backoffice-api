package erp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	domainsales "crm-core/domain/sales"
)

// salesOrderWire mirrors the JSON payload of erp-api-v2
// GET /api/v1/crm/sales-analytics. It keeps the transport shape out of the
// domain package.
type salesOrderWire struct {
	Platform        string   `json:"platform"`
	OrderNo         string   `json:"orderNo"`
	OrderedAt       string   `json:"orderedAt"`
	HasTime         bool     `json:"hasTime"`
	BuyerID         string   `json:"buyerId"`
	DateDay         string   `json:"date"`
	Amount          float64  `json:"amount"`
	Buyer           string   `json:"buyer"`
	Channel         string   `json:"channel"`
	ChannelType     string   `json:"channelType"`
	Province        string   `json:"province"`
	Sets            []string `json:"sets"`
	Items           []string `json:"items"`
	ChickenBags     int      `json:"chickenBags"`
	TunaBags        int      `json:"tunaBags"`
	RabbitBags      int      `json:"rabbitBags"`
	TotalBags       int      `json:"totalBags"`
	CatWand         int      `json:"catWand"`
	ReplacementHead int      `json:"replacementHead"`
	Catnip          int      `json:"catnip"`
	Lines           []struct {
		SKU    string  `json:"sku"`
		Name   string  `json:"name"`
		Qty    int     `json:"qty"`
		Amount float64 `json:"amount"`
		Flavor string  `json:"flavor"`
	} `json:"lines"`
}

type salesOrdersPayload struct {
	Orders []salesOrderWire `json:"orders"`
}

// SalesOrders fetches the order facts of one range from erp-api-v2. Date bounds
// are YYYY-MM-DD in Bangkok time; to is inclusive. It forwards the staff token
// so the ERP applies its own role checks.
func (c *Client) SalesOrders(ctx context.Context, bearerToken string, from, to time.Time, channel, region string) ([]domainsales.SalesOrder, error) {
	q := url.Values{}
	q.Set("dateFrom", from.In(bangkokZone()).Format("2006-01-02"))
	q.Set("dateTo", to.In(bangkokZone()).AddDate(0, 0, -1).Format("2006-01-02"))
	if channel != "" {
		q.Set("channel", channel)
	}
	if region != "" {
		q.Set("region", region)
	}

	endpoint := c.baseURL + "/crm/sales-analytics?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, ErrUnauthorized
	case resp.StatusCode == http.StatusBadRequest:
		return nil, fmt.Errorf("%w: the ERP rejected the date range", ErrUnavailable)
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("%w: status %d", ErrUnavailable, resp.StatusCode)
	}

	var body envelope[salesOrdersPayload]
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("%w: decode: %v", ErrUnavailable, err)
	}
	if !body.Success {
		return nil, fmt.Errorf("%w: unexpected payload", ErrUnavailable)
	}

	out := make([]domainsales.SalesOrder, 0, len(body.Data.Orders))
	for _, w := range body.Data.Orders {
		o := domainsales.SalesOrder{
			Platform:        w.Platform,
			OrderNo:         w.OrderNo,
			OrderedAt:       w.OrderedAt,
			HasTime:         w.HasTime,
			BuyerID:         w.BuyerID,
			DateDay:         w.DateDay,
			Amount:          w.Amount,
			Buyer:           w.Buyer,
			Channel:         w.Channel,
			ChannelType:     w.ChannelType,
			Province:        w.Province,
			Sets:            w.Sets,
			Items:           w.Items,
			ChickenBags:     w.ChickenBags,
			TunaBags:        w.TunaBags,
			RabbitBags:      w.RabbitBags,
			TotalBags:       w.TotalBags,
			CatWand:         w.CatWand,
			ReplacementHead: w.ReplacementHead,
			Catnip:          w.Catnip,
		}
		for _, l := range w.Lines {
			o.Lines = append(o.Lines, domainsales.OrderLine{
				SKU: l.SKU, Name: l.Name, Qty: l.Qty, Amount: l.Amount, Flavor: l.Flavor,
			})
		}
		out = append(out, o)
	}
	return out, nil
}

func bangkokZone() *time.Location {
	return time.FixedZone("Asia/Bangkok", 7*60*60)
}
