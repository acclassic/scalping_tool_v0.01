package binance

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"swap-trader/apintrf"
	"time"
)

type queryParams map[string]string

var client = http.Client{
	Timeout: time.Second * 30,
}

func get_req(endpoint string, params queryParams) *http.Response {
	u := fmt.Sprintf("%s%s", api.Address, endpoint)
	rUrl, _ := url.Parse(u)
	qParams := url.Values{}
	for k, v := range params {
		qParams.Add(k, v)
	}
	rUrl.RawQuery = qParams.Encode()
	req, err := http.NewRequest("GET", rUrl.String(), nil)
	if err != nil {
		apintrf.Log_err().Printf("Error creating request: %s", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		apintrf.Log_err().Printf("Error Response form Server: %s", err)
	}
	return resp
}

func sGet_req(endpoint string, params queryParams) *http.Response {
	u := fmt.Sprintf("%s%s", api.Address, endpoint)
	rUrl, _ := url.Parse(u)
	qParams := url.Values{}
	for k, v := range params {
		qParams.Add(k, v)
	}
	//add timestamp
	qParams.Add("timestamp", get_timestamp())
	//add secure signature
	qParams.Add("signature", get_signature(qParams.Encode()))
	rUrl.RawQuery = qParams.Encode()
	req, err := http.NewRequest("GET", rUrl.String(), nil)
	if err != nil {
		apintrf.Log_err().Printf("Error creating request: %s", err)
	}
	//add header with api key
	header := get_api_header()
	req.Header = header
	resp, err := client.Do(req)
	if err != nil {
		apintrf.Log_err().Printf("Error Response form Server: %s", err)
	}
	return resp
}

func sPost_req(endpoint string, params queryParams) *http.Response {
	u := fmt.Sprintf("%s%s", api.Address, endpoint)
	rUrl, _ := url.Parse(u)
	qParams := url.Values{}
	for k, v := range params {
		qParams.Add(k, v)
	}
	//add timestamp
	qParams.Add("timestamp", get_timestamp())
	//add secure signature
	qParams.Add("signature", get_signature(qParams.Encode()))
	body := strings.NewReader(qParams.Encode())
	req, err := http.NewRequest("POST", rUrl.String(), body)
	if err != nil {
		apintrf.Log_err().Printf("Error creating request: %s", err)
	}
	//add header with api key
	req.Header = get_api_header()
	resp, err := client.Do(req)
	if err != nil {
		apintrf.Log_err().Printf("Error Response form Server: %s", err)
	}
	return resp
}

func get_signature(qParams string) string {
	h := hmac.New(sha256.New, []byte(api.SecretKey))
	h.Write([]byte(qParams))
	signature := fmt.Sprintf("%x", h.Sum(nil))
	return signature
}

func get_timestamp() string {
	t := time.Now().UnixMilli()
	return fmt.Sprint(t)
}

func get_api_header() http.Header {
	header := http.Header{}
	header.Add("X-MBX-APIKEY", api.ApiKey)
	return header
}
