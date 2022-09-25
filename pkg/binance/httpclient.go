package binance

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"swap-trader/pkg/log"
	"time"
)

type queryParams map[string]string

var client = http.Client{
	Timeout: time.Second * 30,
}

//TODO add recvWindow param for safty. See https://binance-docs.github.io/apidocs/spot/en/#signed-trade-user_data-and-margin-endpoint-security

type httpReq struct {
	method  string
	url     string
	qParams queryParams
	body    io.ReadCloser
	secure  bool
	weight  int
}

func create_httpReq(method, url string, qParams queryParams, s bool, weight int) httpReq {
	req := httpReq{
		method:  method,
		url:     url,
		qParams: qParams,
		body:    nil,
		secure:  s,
		weight:  weight,
	}
	return req
}

func http_req_handler(ctx context.Context, reqParams httpReq) (*http.Response, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	//Check if reqWeight has to be subtracted or not
	if ctx.Value(ctxKey("updateCtrs")) == true {
		weightCtr := exLimitsCtrs.reqWeight.get_counter()
		reqCtr := exLimitsCtrs.rawReq.get_counter()
		//Check if ctrs allow req execution. If not don't execute and return error.
		if weightCtr-reqParams.weight >= 0 && reqCtr-1 >= 0 {
			exLimitsCtrs.reqWeight.decrease_counter(reqParams.weight)
			exLimitsCtrs.rawReq.decrease_counter(1)
		} else {
			//TODO see if needs to be logged
			err := errors.New("Counters low. API request could not be executed.")
			return nil, err
		}
	}
	url, body := create_req_params(reqParams)
	req, err := http.NewRequestWithContext(ctx, reqParams.method, url, body)
	if err != nil {
		//TODO see if only log or panic necessary
		log.Sys_logger().Printf("ERROR: Request '%s' could not be created. %s\n", url, err)
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
		log.Sys_logger().Printf("ERROR: Sending request '%s' resulted in an error. %s\n", url, err)
	}
	//Send resp to error_handler if not 200 or return value
	if resp.StatusCode != 200 {
		error_handler(ctx, resp)
		err = fmt.Errorf("ERROR: HTTP response not 200. %s", resp.Status)
		return nil, err
	} else {
		return resp, nil
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

func check_rLimits(weight int) bool {
	counter := exLimitsCtrs.reqWeight.get_counter()
	if counter-weight > 0 {
		return true
	} else {
		return false
	}
}

func error_handler(ctx context.Context, resp *http.Response) {
	if ctx.Err() != nil {
		return
	}
	if resp.StatusCode != 200 {
		switch resp.StatusCode {
		case 429:
			if waitTime := resp.Header.Get("Retry-After"); waitTime != "" {
				log.Sys_logger().Fatalln("WARNING: App stopped. HTTP Resp returned empty 'Retry-After' Header. Check Binance support.")
			} else {
				n, _ := strconv.Atoi(waitTime)
				d := time.Duration(n) * time.Second
				cancelCh <- d
			}
		}
	}
}
