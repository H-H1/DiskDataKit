package main

import (
	"crypto/sha256"
	"fmt"
	"math/rand"
	"time"
)

// generateRandomData 生成指定长度的随机字节数据
func generateRandomData(length int) []byte {
	data := make([]byte, length)
	rand.Read(data)
	return data
}

// getShardIndex 计算哈希值对应的分片索引
func getShardIndex(hash [32]byte, shardCount int) int {
	// 使用哈希值的前几个字节来计算分片索引
	// 这里使用前4个字节转换为uint32，然后取模
	var value uint32
	for i := 0; i < 4; i++ {
		value = (value << 8) | uint32(hash[i])
	}
	return int(value % uint32(shardCount))
}

// testUniformity 测试分片均匀性
func testUniformity(dataSize, shardCount, testCount int) {
	fmt.Printf("\n=== 测试参数 ===\n")
	fmt.Printf("随机数据大小: %d 字节\n", dataSize)
	fmt.Printf("分片数量: %d\n", shardCount)
	fmt.Printf("测试次数: %d\n", testCount)

	// 初始化分片计数器
	shardCounters := make([]int, shardCount)

	// 记录开始时间
	startTime := time.Now()

	// 执行测试
	for i := 0; i < testCount; i++ {
		// 生成随机数据
		data := generateRandomData(dataSize)

		// 计算 SHA256 哈希
		hash := sha256.Sum256(data)

		// 计算分片索引
		index := getShardIndex(hash, shardCount)
		shardCounters[index]++
	}

	elapsed := time.Since(startTime)

	// 统计结果
	fmt.Printf("\n=== 统计结果 ===\n")
	fmt.Printf("执行时间: %v\n", elapsed)
	fmt.Printf("平均每个分片应有数据: %.2f\n", float64(testCount)/float64(shardCount))
	fmt.Printf("每个分片的数据分布:\n")

	minCount := testCount
	maxCount := 0
	totalDeviation := 0.0
	expectedAvg := float64(testCount) / float64(shardCount)

	for i, count := range shardCounters {
		deviation := float64(count) - expectedAvg
		deviationPercent := (deviation / expectedAvg) * 100
		fmt.Printf("分片 %3d: %6d 个 (偏差: %+.2f%%)\n", i, count, deviationPercent)

		if count < minCount {
			minCount = count
		}
		if count > maxCount {
			maxCount = count
		}
		totalDeviation += deviation * deviation
	}

	// 计算标准差
	stdDeviation := totalDeviation / float64(shardCount)

	// 均匀性评估
	fmt.Printf("\n=== 均匀性评估 ===\n")
	fmt.Printf("最小分片数: %d\n", minCount)
	fmt.Printf("最大分片数: %d\n", maxCount)
	fmt.Printf("最大偏差: %+.2f%%\n", (float64(maxCount)-expectedAvg)/expectedAvg*100)
	fmt.Printf("最小偏差: %+.2f%%\n", (float64(minCount)-expectedAvg)/expectedAvg*100)
	fmt.Printf("标准差: %.2f\n", stdDeviation)

	// 卡方检验（粗略评估）
	chiSquare := 0.0
	for _, count := range shardCounters {
		chiSquare += float64(count*count) / expectedAvg
	}
	chiSquare -= float64(testCount)
	fmt.Printf("卡方值: %.2f\n", chiSquare)

	// 判断是否均匀（95%置信度下的粗略判断）
	if chiSquare < float64(shardCount)*1.5 {
		fmt.Println("✅ 分布较为均匀")
	} else if chiSquare < float64(shardCount)*2.5 {
		fmt.Println("⚠️ 分布基本均匀，但有一定偏差")
	} else {
		fmt.Println("❌ 分布不均匀，建议调整")
	}
}

func main() {
	// 设置随机种子
	rand.Seed(time.Now().UnixNano())

	// 可以在这里修改参数
	dataSize := 64      // 随机数据的大小（字节）
	shardCount := 10    // 分片数量
	testCount := 100000 // 测试次数

	// 也可以使用命令行参数或者交互式输入
	// 这里直接使用变量配置

	fmt.Println("SHA256 哈希分片均匀性测试")
	fmt.Println("================================")

	// 运行测试
	testUniformity(dataSize, shardCount, testCount)

	// 额外测试：不同数据大小的影响
	fmt.Println("\n\n=== 不同数据大小对比测试 ===")
	sizes := []int{16, 64, 256, 1024}
	for _, size := range sizes {
		testUniformity(size, shardCount, 10000)
	}

	// 额外测试：不同分片数量的影响
	fmt.Println("\n\n=== 不同分片数量对比测试 ===")
	shards := []int{5, 10, 20, 50}
	for _, shard := range shards {
		testUniformity(dataSize, shard, 10000)
	}
}
