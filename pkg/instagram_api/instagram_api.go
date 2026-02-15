package instagram_api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const maxAPIResponseSize = 1 * 1024 * 1024 // 1 MB for API responses

func getBaseURL() string {
	if url := os.Getenv("INSTAGRAM_API_BASE_URL"); url != "" {
		return strings.TrimRight(url, "/")
	}
	return "https://api.hikerapi.com"
}

type InstagramApi struct {
	client *http.Client
	token  string
}

type GetProfileA1Response struct {
	Graphql struct {
		User struct {
			ProfilePic string `json:"profile_pic_url_hd"`
		} `json:"user"`
	} `json:"graphql"`
}

type GetProfileV1Response struct {
	Pk                       any         `json:"pk"`
	Username                 string      `json:"username"`
	FullName                 string      `json:"full_name"`
	IsPrivate                bool        `json:"is_private"`
	ProfilePicUrl            string      `json:"profile_pic_url"`
	ProfilePicUrlHd          string      `json:"profile_pic_url_hd"`
	IsVerified               bool        `json:"is_verified"`
	MediaCount               int         `json:"media_count"`
	FollowerCount            int         `json:"follower_count"`
	FollowingCount           int         `json:"following_count"`
	Biography                string      `json:"biography"`
	ExternalUrl              string      `json:"external_url"`
	AccountType              int         `json:"account_type"`
	IsBusiness               bool        `json:"is_business"`
	PublicEmail              string      `json:"public_email"`
	ContactPhoneNumber       string      `json:"contact_phone_number"`
	PublicPhoneCountryCode   string      `json:"public_phone_country_code"`
	PublicPhoneNumber        string      `json:"public_phone_number"`
	BusinessContactMethod    string      `json:"business_contact_method"`
	BusinessCategoryName     interface{} `json:"business_category_name"`
	CategoryName             interface{} `json:"category_name"`
	Category                 string      `json:"category"`
	AddressStreet            string      `json:"address_street"`
	CityId                   interface{} `json:"city_id"`
	CityName                 string      `json:"city_name"`
	Latitude                 float64     `json:"latitude"`
	Longitude                float64     `json:"longitude"`
	Zip                      string      `json:"zip"`
	InstagramLocationId      string      `json:"instagram_location_id"`
	InteropMessagingUserFbid interface{} `json:"interop_messaging_user_fbid"`
}

func New(token string) *InstagramApi {
	if token == "" {
		log.Printf("⚠️ InstagramApi created with empty token")
	}

	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	api := InstagramApi{
		client,
		token,
	}

	return &api
}

func (h *InstagramApi) getProfileV1(username string) (GetProfileV1Response, error) {
	reqContext, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	req, err := http.NewRequestWithContext(reqContext, "GET", getBaseURL()+"/v1/user/by/username", nil)
	if err != nil {
		return GetProfileV1Response{}, err
	}

	q := req.URL.Query()
	q.Add("username", username)
	req.URL.RawQuery = q.Encode()

	req.Header.Set("x-access-key", h.token)

	res, err := h.client.Do(req)
	if err != nil {
		return GetProfileV1Response{}, err
	}

	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, maxAPIResponseSize))
	if err != nil {
		return GetProfileV1Response{}, err
	}

	if res.StatusCode != 200 {
		return GetProfileV1Response{}, errors.New(string(body))
	}

	var response GetProfileV1Response
	if err := json.Unmarshal(body, &response); err != nil {
		return GetProfileV1Response{}, err
	}

	return response, nil
}

func (h *InstagramApi) GetProfile(username string) (string, error) {
	res, err := h.getProfileV1(username)
	if err != nil {
		return "", err
	}

	// First check for HD pic
	if res.ProfilePicUrlHd != "" {
		return res.ProfilePicUrlHd, nil
	}

	// If no HD pic, check for regular pic
	if res.ProfilePicUrl != "" {
		return res.ProfilePicUrl, nil
	}

	// Both are empty, return error
	log.Printf("Both profile picture URLs are empty for user: %s", username)
	return "", errors.New("profile picture URL is empty")
}
