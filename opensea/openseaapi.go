package opensea

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

type endpoint string

const (
	retrieveSingleContractEndpoint  endpoint = "api/v1/asset_contract/{id}"
	retrieveCollectionStatsEndpoint endpoint = "api/v1/collection/{id}/stats"

	idTag = "{id}"
)

func (e endpoint) url(host string) string {
	return fmt.Sprintf("%s/%s", host, string(e))
}

func (e endpoint) urlID(host, id string) string {
	u := fmt.Sprintf("%s/%s", host, string(e))
	return strings.ReplaceAll(u, idTag, id)
}

type Client struct {
	Authorizer string
	Client     *http.Client
	Host       string
}

// Error is part of the HTTP response error
type Error struct {
	Parameters interface{} `json:"parameters"`
	Message    string      `json:"message"`
}

// HTTPError is a response error where the body is not JSON, but XML.  This commonly seen in 404 errors.
type HTTPError struct {
	Status     string
	StatusCode int
	URL        string
}

func (h *HTTPError) Error() string {
	return fmt.Sprintf("twitter [%s] status: %s code: %d", h.URL, h.Status, h.StatusCode)
}

// ErrorResponse is returned by a non-success callout
type ErrorResponse struct {
	StatusCode int
	Errors     []Error `json:"errors"`
	Title      string  `json:"title"`
	Detail     string  `json:"detail"`
	Type       string  `json:"type"`
}

func (e *ErrorResponse) Error() string {
	return fmt.Sprintf("opensea callout status %d %s:%s", e.StatusCode, e.Title, e.Detail)
}

// ErrParameter will indicate that the error is from an invalid input parameter
var ErrParameter = errors.New("opensea input parameter error")

type OpenSeaCollection struct {
	Collection struct {
		BannerImageURL          string `json:"banner_image_url"`
		ChatURL                 string `json:"chat_url"`
		CreatedDate             string `json:"created_date"`
		DefaultToFiat           bool   `json:"default_to_fiat"`
		Description             string `json:"description"`
		DevBuyerFeeBasisPoints  string `json:"dev_buyer_fee_basis_points"`
		DevSellerFeeBasisPoints string `json:"dev_seller_fee_basis_points"`
		DiscordURL              string `json:"discord_url"`
		DisplayData             struct {
			CardDisplayStyle string `json:"card_display_style"`
		} `json:"display_data"`
		ExternalURL                string `json:"external_url"`
		Featured                   bool   `json:"featured"`
		FeaturedImageURL           string `json:"featured_image_url"`
		Hidden                     bool   `json:"hidden"`
		SafelistRequestStatus      string `json:"safelist_request_status"`
		ImageURL                   string `json:"image_url"`
		IsSubjectToWhitelist       bool   `json:"is_subject_to_whitelist"`
		LargeImageURL              string `json:"large_image_url"`
		MediumUsername             string `json:"medium_username"`
		Name                       string `json:"name"`
		OnlyProxiedTransfers       bool   `json:"only_proxied_transfers"`
		OpenseaBuyerFeeBasisPoints string `json:"opensea_buyer_fee_basis_points"`
		//OpenseaSellerFeeBasisPoints string `json:"opensea_seller_fee_basis_points"`
		OpenseaSellerFeeBasisPoints int    `json:"opensea_seller_fee_basis_points"`
		PayoutAddress               string `json:"payout_address"`
		RequireEmail                bool   `json:"require_email"`
		ShortDescription            string `json:"short_description"`
		Slug                        string `json:"slug"`
		TelegramURL                 string `json:"telegram_url"`
		TwitterUsername             string `json:"twitter_username"`
		InstagramUsername           string `json:"instagram_username"`
		WikiURL                     string `json:"wiki_url"`
	} `json:"collection"`
	Address                     string `json:"address"`
	AssetContractType           string `json:"asset_contract_type"`
	CreatedDate                 string `json:"created_date"`
	Name                        string `json:"name"`
	NftVersion                  string `json:"nft_version"`
	OpenseaVersion              string `json:"opensea_version"`
	Owner                       int    `json:"owner"`
	SchemaName                  string `json:"schema_name"`
	Symbol                      string `json:"symbol"`
	TotalSupply                 string `json:"total_supply"`
	Description                 string `json:"description"`
	ExternalLink                string `json:"external_link"`
	ImageURL                    string `json:"image_url"`
	DefaultToFiat               bool   `json:"default_to_fiat"`
	DevBuyerFeeBasisPoints      int    `json:"dev_buyer_fee_basis_points"`
	DevSellerFeeBasisPoints     int    `json:"dev_seller_fee_basis_points"`
	OnlyProxiedTransfers        bool   `json:"only_proxied_transfers"`
	OpenseaBuyerFeeBasisPoints  int    `json:"opensea_buyer_fee_basis_points"`
	OpenseaSellerFeeBasisPoints int    `json:"opensea_seller_fee_basis_points"`
	BuyerFeeBasisPoints         int    `json:"buyer_fee_basis_points"`
	SellerFeeBasisPoints        int    `json:"seller_fee_basis_points"`
	PayoutAddress               string `json:"payout_address"`
}

type OpenSeaStats struct {
	Stats struct {
		OneDayVolume          float64 `json:"one_day_volume"`
		OneDayChange          float64 `json:"one_day_change"`
		OneDaySales           float64 `json:"one_day_sales"`
		OneDayAveragePrice    float64 `json:"one_day_average_price"`
		SevenDayVolume        float64 `json:"seven_day_volume"`
		SevenDayChange        float64 `json:"seven_day_change"`
		SevenDaySales         float64 `json:"seven_day_sales"`
		SevenDayAveragePrice  float64 `json:"seven_day_average_price"`
		ThirtyDayVolume       float64 `json:"thirty_day_volume"`
		ThirtyDayChange       float64 `json:"thirty_day_change"`
		ThirtyDaySales        float64 `json:"thirty_day_sales"`
		ThirtyDayAveragePrice float64 `json:"thirty_day_average_price"`
		TotalVolume           float64 `json:"total_volume"`
		TotalSales            float64 `json:"total_sales"`
		TotalSupply           float64 `json:"total_supply"`
		Count                 float64 `json:"count"`
		NumOwners             int     `json:"num_owners"`
		AveragePrice          float64 `json:"average_price"`
		NumReports            int     `json:"num_reports"`
		MarketCap             float64 `json:"market_cap"`
		FloorPrice            float64 `json:"floor_price"`
	} `json:"stats"`
}

func (c *Client) AssetContract(ctx context.Context, id string) (*OpenSeaCollection, error) {
	if len(id) == 0 {
		return nil, fmt.Errorf("collection stats: id is required: %w", ErrParameter)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, retrieveSingleContractEndpoint.urlID(c.Host, id), nil)
	if err != nil {
		return nil, fmt.Errorf("collection stats: request: %w", err)
	}
	req.Header.Add("Accept", "application/json")
	if c.Authorizer != "" {
		req.Header.Add("X-API-KEY", c.Authorizer)
	}
	//c.Authorizer.Add(req)
	//opts.addQuery(req)
	q := req.URL.Query()
	req.URL.RawQuery = q.Encode()

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("collection stats response: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("collection stats response read: %w", err)
	}
	//body := string(respBytes)
	//fmt.Println(body)
	if resp.StatusCode != http.StatusOK {
		e := &ErrorResponse{}
		if err := json.Unmarshal(respBytes, e); err != nil {
			return nil, &HTTPError{
				Status:     resp.Status,
				StatusCode: resp.StatusCode,
				URL:        resp.Request.URL.String(),
			}
		}
		e.StatusCode = resp.StatusCode
		return nil, e
	}

	collection := &OpenSeaCollection{}

	if err := json.Unmarshal(respBytes, collection); err != nil {
		return nil, fmt.Errorf("collection stats raw response error decode: %w", err)
	}

	return collection, nil
}

func (c *Client) CollectionStats(ctx context.Context, id string) (*OpenSeaStats, error) {
	if len(id) == 0 {
		return nil, fmt.Errorf("collection stats: id is required: %w", ErrParameter)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, retrieveCollectionStatsEndpoint.urlID(c.Host, id), nil)
	if err != nil {
		return nil, fmt.Errorf("collection stats: request: %w", err)
	}
	req.Header.Add("Accept", "application/json")
	if c.Authorizer != "" {
		req.Header.Add("X-API-KEY", c.Authorizer)
	}
	//c.Authorizer.Add(req)
	//opts.addQuery(req)
	q := req.URL.Query()
	req.URL.RawQuery = q.Encode()

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("collection stats response: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("collection stats response read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		e := &ErrorResponse{}
		if err := json.Unmarshal(respBytes, e); err != nil {
			return nil, &HTTPError{
				Status:     resp.Status,
				StatusCode: resp.StatusCode,
				URL:        resp.Request.URL.String(),
			}
		}
		e.StatusCode = resp.StatusCode
		return nil, e
	}

	stats := &OpenSeaStats{}

	if err := json.Unmarshal(respBytes, stats); err != nil {
		return nil, fmt.Errorf("collection stats raw response error decode: %w", err)
	}

	return stats, nil
}
