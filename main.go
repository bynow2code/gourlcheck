package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

type CheckResult struct {
	Url    string        //要检测的url
	Code   int           //响应状态码
	Cost   time.Duration //花费时间
	ErrMsg string        //失败原因，空则成功
}

var version = "0.0.0-dev"

func main() {
	concurrency := flag.Int("c", 5, "并发数（默认5）")
	timeout := flag.Int("t", 5, "超时时间（秒，默认5）")
	inputFile := flag.String("f", "", "URL列表文件路径（每行一个URL）")
	outputFile := flag.String("o", "", "结果导出为CSV文件路径（如 -o result.csv）")
	flag.Parse()

	var urls []string
	var err error
	if *inputFile != "" {
		urls, err = readURLsFromFile(*inputFile)
		if err != nil {
			fmt.Printf("读取文件错误：[%s]", err)
			return
		}
	} else {
		urls = flag.Args()
	}

	if len(urls) == 0 {
		fmt.Println("请传入要检测的URL，示例：go run main.go https://baidu.com https://github.com")
		return
	}

	sem := make(chan struct{}, *concurrency)
	resultCh := make(chan CheckResult, len(urls))
	var wg sync.WaitGroup

	for _, url := range urls {
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer func() {
				<-sem
				wg.Done()
			}()

			result := checkSingleURL(url, *timeout)
			resultCh <- result
		}()
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	var results []CheckResult
	for result := range resultCh {
		results = append(results, result)
	}

	if *outputFile != "" {
		err = exportToCSV(results, *outputFile)
		if err != nil {
			fmt.Printf("导出CSV失败：[%s]\n", err)
			return
		}
	} else {
		// 打印结果
		for _, result := range results {
			if result.ErrMsg == "" {
				fmt.Printf(" ✅ URL:[%s] 状态码:[%d] 耗时:[%.2f ms] 失败原因:[%s]\n", result.Url, result.Code, result.Cost.Seconds()*1000, result.ErrMsg)
			} else {
				fmt.Printf(" ❌ URL:[%s] 状态码:[%d] 耗时:[%.2f ms] 失败原因:[%s]\n", result.Url, result.Code, result.Cost.Seconds()*1000, result.ErrMsg)
			}
		}
	}
}

func exportToCSV(results []CheckResult, filePath string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	w := csv.NewWriter(file)
	defer w.Flush()
	err = w.Write([]string{"URL", "状态码", "耗时(ms)", "失败原因"})
	if err != nil {
		return err
	}

	for _, result := range results {
		err = w.Write([]string{
			result.Url,
			fmt.Sprintf("%d", result.Code),
			fmt.Sprintf("%.2f", result.Cost.Seconds()*1000),
			result.ErrMsg,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func readURLsFromFile(filePath string) ([]string, error) {
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var urls []string
	start := 0
	content := string(bytes)
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			line := content[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			if len(line) > 0 {
				urls = append(urls, line)
			}
			start = i + 1
		}
	}

	// 处理最后一行
	if start < len(content) {
		line := content[start:]
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		if len(line) > 0 {
			urls = append(urls, line)
		}
	}

	return urls, nil
}

func checkSingleURL(url string, timeout int) CheckResult {
	result := CheckResult{
		Url:    url,
		Code:   0,
		Cost:   0,
		ErrMsg: "",
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
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
