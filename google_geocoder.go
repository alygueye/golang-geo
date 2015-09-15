package geo

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

type GoogleAuthSchema int

const (
	_ = iota
	GoogleMapsAPIToken
	GoogleMapsForWorkAuth
)

// This struct contains all the funcitonality
// of interacting with the Google Maps Geocoding Service
type GoogleGeocoder struct {
	HttpClient *http.Client
	AuthSchema GoogleAuthSchema
}

// This struct contains selected fields from Google's Geocoding Service response
type googleGeocodeResponse struct {
	Results []struct {
		FormattedAddress string `json:"formatted_address"`
		Geometry         struct {
			Location struct {
				Lat float64
				Lng float64
			}
		}
	}
}

type googleReverseGeocodeResponse struct {
	Results []struct {
		FormattedAddress string `json:"formatted_address"`
	}
}

// This is the error that consumers receive when there
// are no results from the geocoding request.
var googleZeroResultsError = errors.New("ZERO_RESULTS")

// This contains the base URL for the Google Geocoder API.
var googleGeocodeURL = "https://maps.googleapis.com/maps/api/geocode/json"

var GoogleAPIKey = ""

var GoogleClientID = ""
var GooglePrivateKey = ""
var GoogleChannel = ""

// Note:  In the next major revision (1.0.0), it is planned
//        That Geocoders should adhere to the `geo.Geocoder`
//        interface and provide versioning of APIs accordingly.
// Sets the base URL for the Google Geocoding API.
func SetGoogleGeocodeURL(newGeocodeURL string) {
	googleGeocodeURL = newGeocodeURL
}

func SetGoogleAPIKey(newAPIKey string) {
	GoogleAPIKey = newAPIKey
}

func SetGoogleClientID(newGoogleClientID string) {
	GoogleClientID = newGoogleClientID
}

func SetGooglePrivateKey(newGooglePrivateKey string) {
	GooglePrivateKey = newGooglePrivateKey
}

func SetGoogleChannel(newGoogleChannel string) {
	GoogleChannel = newGoogleChannel
}

// Issues a request to the google geocoding service and forwards the passed in params string
// as a URL-encoded entity.  Returns an array of byes as a result, or an error if one occurs during the process.
// Note: Since this is an arbitrary request, you are responsible for passing in your API key if you want one.
func (g *GoogleGeocoder) Request(params string) ([]byte, error) {
	if g.HttpClient == nil {
		g.HttpClient = &http.Client{}
	}

	client := g.HttpClient

	fullUrl := fmt.Sprintf("%s?%s", googleGeocodeURL, params)

	// TODO Potentially refactor out from MapQuestGeocoder as well
	req, _ := http.NewRequest("GET", fullUrl, nil)
	resp, requestErr := client.Do(req)

	if requestErr != nil {
		return nil, requestErr
	}

	data, dataReadErr := ioutil.ReadAll(resp.Body)

	if dataReadErr != nil {
		return nil, dataReadErr
	}

	return data, nil
}

// Geocodes the passed in query string and returns a pointer to a new Point struct.
// Returns an error if the underlying request cannot complete.
func (g *GoogleGeocoder) Geocode(address string) (*Point, error) {
	params := googleGeocodeQueryStr(address)

	queryStr, err := g.googleFormattedRequestStr(params)
	if err != nil {
		return nil, err
	}

	data, err := g.Request(queryStr)
	if err != nil {
		return nil, err
	}

	res := &googleGeocodeResponse{}
	json.Unmarshal(data, res)

	if len(res.Results) == 0 {
		return nil, googleZeroResultsError
	}

	lat := res.Results[0].Geometry.Location.Lat
	lng := res.Results[0].Geometry.Location.Lng

	point := &Point{
		lat: lat,
		lng: lng,
	}

	return point, nil
}

func (g *GoogleGeocoder) googleFormattedRequestStr(params string) (string, error) {
	query := fmt.Sprintf("%s&%s", "sensor=false", params)

	switch g.AuthSchema {
	case GoogleMapsAPIToken:
		return buildGoogleMapsClientSideQuery(query)
	case GoogleMapsForWorkAuth:
		return buildGoogleMapsForWorkQuery(query)
	default:
		return buildDefaultGoogleMapsQuery(query)
	}
}

func buildGoogleMapsClientSideQuery(query string) (string, error) {
	queryBuffer := bytes.NewBufferString(query)

	_, err := queryBuffer.WriteString(fmt.Sprintf("&key=%s", GoogleAPIKey))
	if err != nil {
		return "", err
	}

	return queryBuffer.String(), nil
}

func buildGoogleMapsForWorkQuery(query string) (string, error) {
	queryBuffer := bytes.NewBufferString(query)

	if GoogleChannel != "" {
		_, err := queryBuffer.WriteString(fmt.Sprintf("&channel=%s", GoogleChannel))
		if err != nil {
			return "", err
		}
	}

	_, err := queryBuffer.WriteString(fmt.Sprintf("&client=%s", GoogleClientID))
	if err != nil {
		return "", err
	}

	u, err := url.Parse(fmt.Sprintf("%s?%s", googleGeocodeURL, queryBuffer.String()))
	if err != nil {
		return "", err
	}

	requestUri := u.RequestURI()

	decodedKey, err := base64.URLEncoding.DecodeString(GooglePrivateKey)
	if err != nil {
		return "", err
	}

	mac := hmac.New(sha1.New, decodedKey)
	_, err = mac.Write([]byte(requestUri))

	encodedSignature := base64.URLEncoding.EncodeToString(mac.Sum(nil))

	return fmt.Sprintf("%s&signature=%s", queryBuffer.String(), encodedSignature), nil
}

func buildDefaultGoogleMapsQuery(query string) (string, error) {
	return query, nil
}

func googleGeocodeQueryStr(address string) string {
	url_safe_query := url.QueryEscape(address)

	return fmt.Sprintf("address=%s", url_safe_query)
}

// Reverse geocodes the pointer to a Point struct and returns the first address that matches
// or returns an error if the underlying request cannot complete.
func (g *GoogleGeocoder) ReverseGeocode(p *Point) (string, error) {
	params := googleReverseGeocodeQueryStr(p)

	queryStr, err := g.googleFormattedRequestStr(params)
	if err != nil {
		return "", err
	}

	data, err := g.Request(queryStr)
	if err != nil {
		return "", err
	}

	res := &googleReverseGeocodeResponse{}
	err = json.Unmarshal(data, res)
	if err != nil {
		return "", err
	}

	if len(res.Results) == 0 {
		return "", googleZeroResultsError
	}

	return res.Results[0].FormattedAddress, err
}

func googleReverseGeocodeQueryStr(p *Point) string {
	return fmt.Sprintf("latlng=%f,%f", p.lat, p.lng)
}
