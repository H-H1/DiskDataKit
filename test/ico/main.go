package main

import (
	"bufio"
	"encoding/binary"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

// ICO 支持的尺寸
var sizes = []int{256, 128, 64, 48, 32, 16}

func main() {
	if len(os.Args) < 2 {
		println("用法: go run main.go <png文件路径>")
		println("示例: go run main.go ico.png")
		return
	}

	pngPath := os.Args[1]

	// 打开 PNG
	f, err := os.Open(pngPath)
	if err != nil {
		println("打开文件失败:", err.Error())
		return
	}
	defer f.Close()

	src, err := png.Decode(f)
	if err != nil {
		println("解码PNG失败:", err.Error())
		return
	}

	// 生成 ICO 路径
	ext := filepath.Ext(pngPath)
	icoPath := strings.TrimSuffix(pngPath, ext) + ".ico"

	// 生成多尺寸 ICO
	if err := writeICO(icoPath, src); err != nil {
		println("生成ICO失败:", err.Error())
		return
	}

	info, _ := os.Stat(icoPath)
	println("转换成功:", icoPath, "大小:", info.Size(), "bytes")
}

// writeICO 生成包含多个尺寸的标准 ICO 文件（PNG 嵌入格式）。
// Windows Vista+ 支持 ICO 内嵌 PNG，兼容性好且文件小。
func writeICO(path string, src image.Image) error {
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	// 收集各尺寸的 PNG 数据
	type entry struct {
		size int
		data []byte
	}
	var entries []entry

	for _, sz := range sizes {
		scaled := scaleImage(src, sz)
		var buf strings.Builder
		bw := bufio.NewWriter(&buf)
		if err := png.Encode(bw, scaled); err != nil {
			return err
		}
		bw.Flush()
		entries = append(entries, entry{size: sz, data: []byte(buf.String())})
	}

	count := uint16(len(entries))

	// ICONDIR (6 bytes)
	binary.Write(out, binary.LittleEndian, uint16(0)) // reserved
	binary.Write(out, binary.LittleEndian, uint16(1)) // type = ICO
	binary.Write(out, binary.LittleEndian, count)     // image count

	// 计算各 entry 数据偏移
	headerSize := 6 + int(count)*16
	offset := headerSize

	// ICONDIRENTRY (16 bytes each)
	for _, e := range entries {
		sz := byte(e.size)
		if e.size >= 256 {
			sz = 0 // 256 用 0 表示
		}
		out.Write([]byte{sz, sz, 0, 0})                             // width, height, palette, reserved
		binary.Write(out, binary.LittleEndian, uint16(1))           // color planes
		binary.Write(out, binary.LittleEndian, uint16(32))          // bits per pixel
		binary.Write(out, binary.LittleEndian, uint32(len(e.data))) // data size
		binary.Write(out, binary.LittleEndian, uint32(offset))      // data offset
		offset += len(e.data)
	}

	// 写入各尺寸 PNG 数据
	for _, e := range entries {
		out.Write(e.data)
	}

	return nil
}

// scaleImage 将图片缩放到指定尺寸（最近邻，简单快速）。
func scaleImage(src image.Image, size int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	sx := float64(src.Bounds().Dx()) / float64(size)
	sy := float64(src.Bounds().Dy()) / float64(size)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			px := int(float64(x)*sx + 0.5)
			py := int(float64(y)*sy + 0.5)
			if px >= src.Bounds().Dx() {
				px = src.Bounds().Dx() - 1
			}
			if py >= src.Bounds().Dy() {
				py = src.Bounds().Dy() - 1
			}
			dst.Set(x, y, src.At(src.Bounds().Min.X+px, src.Bounds().Min.Y+py))
		}
	}
	return dst
}
