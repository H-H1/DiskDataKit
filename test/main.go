package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// 方法1：使用filepath.Walk单线程遍历
func getDirSizeWalk(dir string) (int64, error) {
	var size int64
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// 方法2：并发统计（多worker处理）
func getDirSizeConcurrent(dir string) (int64, error) {
	var size int64
	var mu sync.Mutex
	var wg sync.WaitGroup

	fileChan := make(chan string, 1000)

	numWorkers := runtime.NumCPU()
	wg.Add(numWorkers)

	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()
			for path := range fileChan {
				info, err := os.Lstat(path)
				if err != nil {
					continue
				}
				if !info.IsDir() {
					mu.Lock()
					size += info.Size()
					mu.Unlock()
				}
			}
		}()
	}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			fileChan <- path
		}
		return nil
	})

	close(fileChan)
	wg.Wait()

	return size, err
}

// 方法3：使用filepath.WalkDir (Go 1.16+)
func getDirSizeWalkDir(dir string) (int64, error) {
	var size int64
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// 方法4：使用WalkDir + 并发
func getDirSizeConcurrentWalkDir(dir string) (int64, error) {
	var size int64
	var mu sync.Mutex
	var wg sync.WaitGroup

	fileChan := make(chan string, 1000)

	numWorkers := runtime.NumCPU()
	wg.Add(numWorkers)

	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()
			for path := range fileChan {
				info, err := os.Lstat(path)
				if err != nil {
					continue
				}
				if !info.IsDir() {
					mu.Lock()
					size += info.Size()
					mu.Unlock()
				}
			}
		}()
	}

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			fileChan <- path
		}
		return nil
	})

	close(fileChan)
	wg.Wait()

	return size, err
}

// 格式化文件大小
func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// 统计函数执行时间
func measureTime(fn func() (int64, error)) (int64, time.Duration) {
	start := time.Now()
	size, err := fn()
	if err != nil {
		fmt.Printf("错误: %v\n", err)
		return 0, 0
	}
	return size, time.Since(start)
}

func main() {
	dir := flag.String("path", `C:\Users\1\AppData\Local\go-build`, "要统计的文件夹路径")
	method := flag.String("method", "all", "统计方法: walk, concurrent, walkdir, concurrent-walkdir, all")
	flag.Parse()

	fmt.Printf("统计文件夹: %s\n", *dir)
	fmt.Printf("CPU核心数: %d\n", runtime.NumCPU())
	fmt.Println(strings.Repeat("-", 60))

	if _, err := os.Stat(*dir); os.IsNotExist(err) {
		fmt.Printf("错误: 目录 %s 不存在\n", *dir)
		return
	}

	switch *method {
	case "walk":
		fmt.Println("方法1: filepath.Walk (单线程)")
		size, duration := measureTime(func() (int64, error) {
			return getDirSizeWalk(*dir)
		})
		fmt.Printf("大小: %s\n", formatSize(size))
		fmt.Printf("耗时: %v\n", duration)

	case "concurrent":
		fmt.Println("方法2: 并发统计 (filepath.Walk + goroutines)")
		size, duration := measureTime(func() (int64, error) {
			return getDirSizeConcurrent(*dir)
		})
		fmt.Printf("大小: %s\n", formatSize(size))
		fmt.Printf("耗时: %v\n", duration)

	case "walkdir":
		fmt.Println("方法3: filepath.WalkDir (Go 1.16+)")
		size, duration := measureTime(func() (int64, error) {
			return getDirSizeWalkDir(*dir)
		})
		fmt.Printf("大小: %s\n", formatSize(size))
		fmt.Printf("耗时: %v\n", duration)

	case "concurrent-walkdir":
		fmt.Println("方法4: 并发 + WalkDir")
		size, duration := measureTime(func() (int64, error) {
			return getDirSizeConcurrentWalkDir(*dir)
		})
		fmt.Printf("大小: %s\n", formatSize(size))
		fmt.Printf("耗时: %v\n", duration)

	case "all":
		fmt.Println("=== 性能对比测试 ===\n")

		fmt.Println("方法1: filepath.Walk (单线程)")
		size1, duration1 := measureTime(func() (int64, error) {
			return getDirSizeWalk(*dir)
		})
		fmt.Printf("  大小: %s\n", formatSize(size1))
		fmt.Printf("  耗时: %v\n\n", duration1)

		fmt.Println("方法2: 并发统计 (filepath.Walk + goroutines)")
		size2, duration2 := measureTime(func() (int64, error) {
			return getDirSizeConcurrent(*dir)
		})
		fmt.Printf("  大小: %s\n", formatSize(size2))
		fmt.Printf("  耗时: %v\n\n", duration2)

		fmt.Println("方法3: filepath.WalkDir (Go 1.16+)")
		size3, duration3 := measureTime(func() (int64, error) {
			return getDirSizeWalkDir(*dir)
		})
		fmt.Printf("  大小: %s\n", formatSize(size3))
		fmt.Printf("  耗时: %v\n\n", duration3)

		fmt.Println("方法4: 并发 + WalkDir")
		size4, duration4 := measureTime(func() (int64, error) {
			return getDirSizeConcurrentWalkDir(*dir)
		})
		fmt.Printf("  大小: %s\n", formatSize(size4))
		fmt.Printf("  耗时: %v\n\n", duration4)

		fmt.Println(strings.Repeat("-", 60))
		fmt.Println("验证结果一致性:")
		sizes := []int64{size1, size2, size3, size4}
		allEqual := true
		for i := 1; i < len(sizes); i++ {
			if sizes[i] != sizes[0] {
				allEqual = false
				break
			}
		}
		if allEqual {
			fmt.Println("✓ 所有方法统计结果一致")
		} else {
			fmt.Println("✗ 警告: 统计结果不一致!")
			for i, s := range sizes {
				fmt.Printf("  方法%d: %d\n", i+1, s)
			}
		}

		fmt.Println("\n性能对比 (以方法1为基准):")
		if duration1 > 0 {
			fmt.Printf("  方法1 (Walk):         %.2fx\n", 1.0)
			fmt.Printf("  方法2 (Concurrent):   %.2fx\n", float64(duration1)/float64(duration2))
			fmt.Printf("  方法3 (WalkDir):      %.2fx\n", float64(duration1)/float64(duration3))
			fmt.Printf("  方法4 (ConW+WalkDir): %.2fx\n", float64(duration1)/float64(duration4))
		}

	default:
		fmt.Printf("未知方法: %s\n", *method)
		fmt.Println("可用方法: walk, concurrent, walkdir, concurrent-walkdir, all")
	}
}
