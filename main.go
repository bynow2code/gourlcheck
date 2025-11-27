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

// CheckResult 表示单个URL检测的结果信息
// Url: 被检测的URL地址
// Code: HTTP响应状态码，0表示未获取到有效状态码
// Cost: 请求所花费的时间
// ErrMsg: 错误信息，如果为空则表示请求成功
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

	// 使用带缓冲的channel控制最大并发数量
	sem := make(chan struct{}, *concurrency)
	// 创建结果通道用于收集所有检测结果
	resultCh := make(chan CheckResult, len(urls))
	var wg sync.WaitGroup

	// 启动多个goroutine进行并发检测
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

	// 在单独的goroutine中等待所有任务完成，并关闭结果通道
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// 收集所有检测结果
	var results []CheckResult
	for result := range resultCh {
		results = append(results, result)
	}

	// 根据是否指定输出文件决定是打印还是导出结果
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

// exportToCSV 将检查结果导出到CSV文件
// 参数:
//   - results: 要导出的检查结果切片
//   - filePath: 目标CSV文件路径
//
// 返回值:
//   - error: 文件创建或写入过程中可能发生的错误
func exportToCSV(results []CheckResult, filePath string) error {
	// 创建目标文件
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}

	// 创建CSV写入器并确保在函数结束时刷新缓冲区
	w := csv.NewWriter(file)
	defer w.Flush()

	// 写入CSV表头
	err = w.Write([]string{"URL", "状态码", "耗时(ms)", "失败原因"})
	if err != nil {
		return err
	}

	// 遍历检查结果并逐行写入CSV文件
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

// readURLsFromFile 从指定文件路径读取URL列表
// 参数:
//
//	filePath: 要读取的文件路径
//
// 返回值:
//
//	[]string: 从文件中解析出的URL字符串切片
//	error: 读取文件或解析过程中可能发生的错误
func readURLsFromFile(filePath string) ([]string, error) {
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var urls []string
	start := 0
	content := string(bytes)

	// 按行分割内容并处理每一行
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			line := content[start:i]
			// 处理Windows换行符(\r\n)
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			// 只添加非空行
			if len(line) > 0 {
				urls = append(urls, line)
			}
			start = i + 1
		}
	}

	// 处理最后一行（文件末尾没有换行符的情况）
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

// checkSingleURL 检查单个URL的可访问性
// 参数:
//
//	url: 要检查的URL地址
//	timeout: 请求超时时间(秒)
//
// 返回值:
//
//	CheckResult: 包含检查结果的结构体，包含URL、状态码、耗时和错误信息
func checkSingleURL(url string, timeout int) CheckResult {
	result := CheckResult{
		Url:    url,
		Code:   0,
		Cost:   0,
		ErrMsg: "",
	}

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	// 构建HEAD请求
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		result.ErrMsg = fmt.Sprintf("构建请求错误：[%s]", err)
		return result
	}

	// 发送HTTP请求并计算耗时
	client := http.Client{}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		result.ErrMsg = fmt.Sprintf("发送请求错误：[%s]", err)
		return result
	}
	defer resp.Body.Close()

	// 设置响应结果
	result.Code = resp.StatusCode
	result.Cost = time.Since(start)
	return result
}
