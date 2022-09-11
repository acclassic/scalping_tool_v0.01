package binance

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"swap-trader/apintrf"
	"time"
)

type queryParams map[string]string

var client = http.Client{
	Timeout: time.Second * 30,
}

//TODO add recvWindow param for safty. See https://binance-docs.github.io/apidocs/spot/en/#signed-trade-user_data-and-margin-endpoint-security

//TODO use this to check if the weight and req has to be counted against the ctrs
type ctxOrigin string

type httpReq struct {
	method  string
	url     string
	qParams queryParams
	body    io.ReadCloser
	secure  bool
	weight  int
}

func http_req_handler(ctx context.Context, reqParams httpReq) {
	//Check if req dosen't come from trade and update counters if not.
	if ctx.Value(ctxOrigin("origin")) == "default" {
		weightCtr := exLimitsCtrs.reqWeight.get_counter()
		reqCtr := exLimitsCtrs.rawReq.get_counter()
		if weightCtr-reqParams.weight >= 0 && reqCtr-1 >= 0 {
			exLimitsCtrs.reqWeight.update_counter(reqParams.weight)
			exLimitsCtrs.rawReq.update_counter(1)
		} else {
			//TODO log this for analytics data
			return
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	url, body := create_req_params(reqParams)
	req, err := http.NewRequestWithContext(ctx, reqParams.method, url, body)
	if err != nil {
		//TODO see if only log or panic necessary
		apintrf.Log_err().Printf("ERROR: Request '%s' could not be created. %s\n", url, err)
	}
	//Add header for secure connection and content-type for post req
	if reqParams.secure == true {
		switch reqParams.method {
		case "GET":
			req.Header = set_get_header()
		case "POST":
			req.Header = set_post_header()
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		apintrf.Log_err().Printf("ERROR: Sending request '%s' resulted in an error. %s\n", url, err)
	}
	http_resp_hander(ctx, resp)
}

func http_resp_hander(ctx context.Context, resp *http.Response) {
	//if ctx.Err() != nil {
	//	fmt.Println(ctx.Err().Error())
	//	return
	//}
	fmt.Println(resp.Status)
	if resp.StatusCode != 200 {
		switch resp.StatusCode {
		case 429:
			if waitTime := resp.Header.Get("Retry-After"); waitTime != "" {
				apintrf.Log_err().Fatalln("WARNING: App stopped. HTTP Resp returned empty 'Retry-After' Header. Check Binance support.")
			} else {
				n, _ := strconv.Atoi(waitTime)
				d := time.Duration(n) * time.Second
				cancelCh <- d
			}
		}
	}
}

func create_req_params(req httpReq) (string, io.Reader) {
	u := fmt.Sprintf("%s%s", api.Address, req.url)
	rUrl, _ := url.Parse(u)
	qParams := url.Values{}
	for k, v := range req.qParams {
		qParams.Add(k, v)
	}
	//Add secure params for signed endpoints
	if req.secure == true {
		qParams.Add("timestamp", set_timestamp())
		qParams.Add("signature", set_signature(qParams.Encode()))
	}
	switch req.method {
	case "GET":
		rUrl.RawQuery = qParams.Encode()
		return rUrl.String(), nil
	case "POST":
		body := strings.NewReader(qParams.Encode())
		return rUrl.String(), body
	default:
		return "", nil
	}
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
	qParams.Add("timestamp", set_timestamp())
	//add secure signature
	qParams.Add("signature", set_signature(qParams.Encode()))
	rUrl.RawQuery = qParams.Encode()
	req, err := http.NewRequest("GET", rUrl.String(), nil)
	if err != nil {
		apintrf.Log_err().Printf("Error creating request: %s", err)
	}
	//add header with api key
	//header := set_api_header()
	//req.Header = header
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
	qParams.Add("timestamp", set_timestamp())
	//add secure signature
	qParams.Add("signature", set_signature(qParams.Encode()))
	body := strings.NewReader(qParams.Encode())
	req, err := http.NewRequest("POST", rUrl.String(), body)
	if err != nil {
		apintrf.Log_err().Printf("Error creating request: %s", err)
	}
	//add header with api key
	//TODO add Content-Type application/x-www-form-urlencoded to header
	//req.Header = set_api_header()
	resp, err := client.Do(req)
	if err != nil {
		apintrf.Log_err().Printf("Error Response form Server: %s", err)
	}
	return resp
}

func set_signature(qParams string) string {
	h := hmac.New(sha256.New, []byte(api.SecretKey))
	h.Write([]byte(qParams))
	signature := fmt.Sprintf("%x", h.Sum(nil))
	return signature
}

func set_timestamp() string {
	t := time.Now().UnixMilli()
	return fmt.Sprint(t)
}

func set_get_header() http.Header {
	header := http.Header{}
	header.Add("X-MBX-APIKEY", api.ApiKey)
	return header
}

func set_post_header() http.Header {
	header := http.Header{}
	header.Add("X-MBX-APIKEY", api.ApiKey)
	header.Add("Content-Type", "application/x-www-form-urlencoded")
	return header
}

func error_handler(resp http.Response) {
	switch resp.StatusCode {
	case 429:
		waitTime := resp.Header.Get("Retry-After")
		if waitTime != "" {
			apintrf.Log_err().Fatalln("WARNING: App stopped. HTTP Resp returned empty 'Retry-After' Header. Check Binance support.")
		} else {
			//d := time.Duration(waitTime) * time.Second
			return
		}
	}
}

func check_rLimits(weight int) bool {
	counter := exLimitsCtrs.reqWeight.get_counter()
	if counter-weight > 0 {
		return true
	} else {
		return false
	}
}
