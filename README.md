# urlcheck：并发URL可用性检测工具

快速检测批量URL的可用性，支持并发控制、超时设置、结果导出CSV。

## 功能
- 从命令行或文件读取URL列表
- 自定义并发数和超时时间
- 输出状态码、响应耗时、失败原因
- 支持结果导出为CSV文件

## 使用示例
```bash
# 检测单个URL
./urlcheck https://baidu.com

# 从文件读取，并发20，超时3秒，导出结果
./urlcheck -f urls.txt -c 20 -t 3 -o result.csv
