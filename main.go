package main

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type CheckResult struct {
	Url    string        //要检测的url
	Code   int           //响应状态码
	Cost   time.Duration //花费时间
	ErrMsg string        //失败原因，空则成功
}

func main() {
	url := "https://baidu.com"
	result := checkSingleURL(url, 1)
	fmt.Printf("url:[%s] code:[%d] cost:[%d] errmsg:[%s]", result.Url, result.Code, result.Cost, result.ErrMsg)
}

func checkSingleURL(url string, timeout int) CheckResult {
	result := CheckResult{
		Url:    url,
		Code:   0,
		Cost:   0,
		ErrMsg: "",
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second*5)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		result.ErrMsg = fmt.Sprintf("构建请求错误：[%s]", err)
		return result
	}

	client := http.Client{}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		result.ErrMsg = fmt.Sprintf("发送请求错误：[%s]", err)
		return result
	}
	defer resp.Body.Close()

	result.Code = resp.StatusCode
	result.Cost = time.Since(start)
	return result
}
