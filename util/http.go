package util

import (
	"context"
	"encoding/json"
	"github.com/morikuni/failure"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Http struct {
	client    *http.Client
	attempts  int
	sleepTime time.Duration // sleepTime will increase with attempt tiems, value = sleepTime * (2 * times - 1)
}

func GetHttp(attempts int, sleepTime time.Duration) Http {
	if attempts < 1 {
		panic("Wrong value of attempts: " + strconv.Itoa(attempts) + ", should >= 1")
	}
	if sleepTime < 0 {
		panic("Wrong value of sleep time: " + sleepTime.String() + ", should >= 0")
	}
	return Http{
		client:    &http.Client{},
		attempts:  attempts,
		sleepTime: sleepTime,
	}
}

func (h *Http) Request(req *http.Request, ctx context.Context, responseStruct interface{}) error {
	req = req.WithContext(ctx)
	escape(req)
	res, err := h.attemptDo(req)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(res.Body)
	responseBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		c := getReqFailureContext(req)
		c["response_body"] = string(responseBody)
		return failure.Wrap(err, c)
	}
	if err = json.Unmarshal(responseBody, &responseStruct); err != nil {
		c := getReqFailureContext(req)
		c["response_body"] = string(responseBody)
		return failure.Wrap(err, c)
	}
	return nil
}

func (h *Http) attemptDo(req *http.Request) (*http.Response, error) {
	var (
		res *http.Response
		err = map[int]error{}
	)
	for i := 0; i < h.attempts; i++ {
		res, err[i] = h.client.Do(req)
		if err[i] != nil {
			if i == h.attempts-1 {
				c := getReqFailureContext(req)
				for k := 0; k < h.attempts-1; k++ {
					c["attempt "+strconv.Itoa(k+1)+" err"] = err[k].Error()
				}
				return res, failure.Wrap(err[i], c)
			} else {
				time.Sleep(h.sleepTime * time.Duration(2*i+1))
				continue
			}
		}
		break
	}
	return res, nil
}

func escape(req *http.Request) {
	req.URL.RawQuery = url.PathEscape(req.URL.RawQuery)
}

func getReqFailureContext(req *http.Request) failure.Context {
	return failure.Context{
		"protocol":    req.Proto,
		"host":        req.URL.Hostname(),
		"port":        req.URL.Port(),
		"request_url": req.URL.RequestURI(),
	}
}