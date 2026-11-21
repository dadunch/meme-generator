package subscription

import "time"

// Plan adalah paket langganan yang tersedia.
type Plan struct {
	ID       string `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	PriceIDR int64  `json:"priceIdr"`
	Duration int32  `json:"durationDays"`
	MaxMemes int32  `json:"maxMemes"`
	Active   bool   `json:"active"`
}

type Subscription struct {
	ID       string     `json:"id"`
	UserID   string     `json:"userId"`
	PlanID   string     `json:"planId"`
	Status   string     `json:"status"`             // pending | active | expired | canceled
	StartsAt *time.Time `json:"startsAt,omitempty"` // NULL saat masih pending
	EndsAt   *time.Time `json:"endsAt,omitempty"`
}

type Payment struct {
	ID             string `json:"id"`
	SubscriptionID string `json:"subscriptionId"`
	AmountIDR      int64  `json:"amountIdr"`
	Status         string `json:"status"` // pending | paid | failed
	Reference      string `json:"reference"`
}

// DTO permintaan/respons.
type CheckoutDTO struct {
	PlanCode string `json:"planCode"`
}

type CheckoutResponse struct {
	Subscription Subscription `json:"subscription"`
	Payment      Payment      `json:"payment"`
	// MockGatewayURL disimulasikan: mengonfirmasi pembayaran secara instan.
	MockGatewayURL string `json:"mockGatewayUrl"`
}

type SubscriptionStatus struct {
	Subscribed bool    `json:"subscribed"`
	Status     string  `json:"status,omitempty"`
	PlanCode   string  `json:"planCode,omitempty"`
	PlanName   string  `json:"planName,omitempty"`
	EndsAt     *string `json:"endsAt,omitempty"`
	MemesUsed  int32   `json:"memesUsed"`
	MaxMemes   int32   `json:"maxMemes"`
}
