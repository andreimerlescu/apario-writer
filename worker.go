/*
Project Apario is the World's Truth Repository that was invented and started by Andrei Merlescu in 2020.
Copyright (C) 2023  Andrei Merlescu

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/
package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/nfnt/resize"
	"github.com/pixiv/go-libjpeg/jpeg"
)

func fileHasData(filename string) (bool, error) {
	sem_filedata.Acquire()
	defer sem_filedata.Release()

	_, existsErr := os.Stat(filename)
	if os.IsNotExist(existsErr) {
		return false, fmt.Errorf("no such file")
	}

	file, err := os.ReadFile(filename)
	if err != nil {
		return false, err
	}

	contents := string(file)
	if len(contents) < 20 {
		regex := regexp.MustCompile("[^a-zA-Z0-9]+")
		filtered := regex.ReplaceAllString(contents, "")
		if len(filtered) > 3 {
			return true, nil
		} else {
			return false, fmt.Errorf("invalid file contents length")
		}
	} else {
		return true, nil
	}

}

func parseDateString(in string) (out time.Time, err error) {
	possibleFormats := []string{
		"01-02-06",
		"01/02/2006",
		"01-02-2006",
		"01/02/2006",
		"2006-01-02T15:04:05-07:00",
	}

	for _, format := range possibleFormats {
		out, err = time.Parse(format, in)
		if err == nil {
			return out, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date string")
}

func compressString(input []byte) ([]byte, error) {
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)

	_, err := gzipWriter.Write(input)
	if err != nil {
		return nil, err
	}

	err = gzipWriter.Close()
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func decompressString(input []byte) ([]byte, error) {
	buf := bytes.NewBuffer(input)
	gzipReader, err := gzip.NewReader(buf)
	if err != nil {
		return nil, err
	}
	defer gzipReader.Close()

	decompressed, err := io.ReadAll(gzipReader)
	if err != nil {
		return nil, err
	}

	return decompressed, nil
}

func generateThreeCharSequences(input string) []Qbit {
	qbitMap := make(map[[3]byte]int)

	for i := 0; i < len(input)-2; i++ {
		var sequence [3]byte
		sequence[0] = input[i]
		sequence[1] = input[i+1]
		sequence[2] = input[i+2]

		qbitMap[sequence]++
	}

	qbits := make([]Qbit, 0, len(qbitMap))
	for seq, count := range qbitMap {
		qbits = append(qbits, Qbit{seq: seq, count: count})
	}

	sort.Slice(qbits, func(i, j int) bool {
		return qbits[i].count > qbits[j].count
	})

	return qbits
}

func IsDir(in string) bool {
	fileInfo, err := os.Stat(in)
	if err != nil {
		if os.IsNotExist(err) {
			return false // File or directory does not exist
		}
		return false // Error occurred while accessing the file or directory
	}

	return fileInfo.IsDir()
}

func FileSha512(file *os.File) (checksum string) {
	sem_shafile.Acquire()
	defer sem_shafile.Release()

	hash := sha512.New()
	if _, err := io.Copy(hash, file); err != nil {
		panic(err)
	}

	checksum = hex.EncodeToString(hash.Sum(nil))
	return checksum
}

func cryptoRandInt(min, max int) (int, error) {
	if min > max {
		return 0, errors.New("invalid range")
	}

	if min == max {
		return min, nil
	}

	n, err := rand.Int(rand.Reader, big.NewInt(int64(max-min+1)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()) + min, nil
}

func downloadFile(ctx context.Context, url string, output string) error {
	var err error
	for i := 0; i < c_retry_attempts; i++ {
		err = tryDownloadFile(ctx, url, output)
		if err == nil {
			return nil
		}

		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			wait, _ := cryptoRandInt(0, 1<<i)
			select {
			case <-time.After(time.Duration(wait) * time.Second):
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		} else {
			log_error.Tracef("downloadFile returned an err: %v", err)
			break
		}
	}
	return err
}

func tryDownloadFile(ctx context.Context, url string, output string) error {
	sem_download.Acquire()
	defer sem_download.Release()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(output)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func Sha256(in string) (checksum string) {
	sem_shastring.Acquire()
	defer sem_shastring.Release()
	hash := sha256.New()
	hash.Write([]byte(in))
	checksum = hex.EncodeToString(hash.Sum(nil))
	return checksum
}

func resizePng(imgFile *os.File, newWidth int, outputFilename string) error {
	sem_resize.Acquire()
	defer sem_resize.Release()

	if newWidth <= 0 {
		return errors.New("invalid width provided")
	}

	// Decode the image using the imaging package
	imgFile.Seek(0, 0) // Make sure the file pointer is at the beginning
	img, err := imaging.Decode(imgFile)
	if err != nil {
		return err
	}

	// Calculate the new height to maintain aspect ratio
	originalBounds := img.Bounds()
	originalWidth := originalBounds.Dx()
	originalHeight := originalBounds.Dy()
	newHeight := int((float64(newWidth) / float64(originalWidth)) * float64(originalHeight))

	// Resize the image using the bilinear interpolation
	newImage := resize.Resize(uint(newWidth), uint(newHeight), img, resize.Bilinear)

	// Create the output file
	outputFile, err := os.Create(outputFilename)
	if err != nil {
		return err
	}
	defer outputFile.Close()

	// Encode the new image as a PNG and save it to the output file
	err = png.Encode(outputFile, newImage)
	if err != nil {
		return err
	}

	return nil
}

func resizeJpg(imgFile *os.File, newWidth int, outputFilename string) error {
	sem_resize.Acquire()
	defer sem_resize.Release()

	if newWidth <= 0 {
		return errors.New("invalid width provided")
	}

	// Decode the image using the imaging package
	imgFile.Seek(0, 0) // Make sure the file pointer is at the beginning
	img, err := imaging.Decode(imgFile)
	if err != nil {
		return err
	}

	// Calculate the new height to maintain aspect ratio
	originalBounds := img.Bounds()
	originalWidth := originalBounds.Dx()
	originalHeight := originalBounds.Dy()
	newHeight := int((float64(newWidth) / float64(originalWidth)) * float64(originalHeight))

	// Resize the image using the bilinear interpolation
	newImage := resize.Resize(uint(newWidth), uint(newHeight), img, resize.Bilinear)

	// Create the output file
	outputFile, err := os.Create(outputFilename)
	if err != nil {
		return err
	}
	defer outputFile.Close()

	// Encode the new image as a progressive JPEG and save it to the output file
	err = jpeg.Encode(outputFile, newImage, &jpeg.EncoderOptions{
		Quality:         *flag_g_jpg_quality,
		OptimizeCoding:  true,
		ProgressiveMode: *flag_g_progressive_jpeg,
	})
	if err != nil {
		return err
	}

	return nil
}

func convertAndOptimizePNG(imgFile *os.File, outputFilename string) error {
	sem_png2jpg.Acquire()
	defer sem_png2jpg.Release()

	imgErr := validatePNGFile(imgFile)
	if imgErr != nil {
		log_error.Trace("convertAndOptimizePNG(imgFile)->validatePNGFile(imgFile) threw err: %+v", imgErr)
		return imgErr
	}

	imgFile.Seek(0, 0)
	img, err := imaging.Decode(imgFile)
	if err != nil {
		return err
	}

	if paletted, ok := img.(*image.Paletted); ok {
		img = palettedToRGBA(paletted)
		log_info.Printf("converting `img` %v *image.Paletted into %T", imgFile.Name(), img)
	}

	if rgba64, ok := img.(*image.RGBA64); ok {
		img = rgba64ToRGBA(rgba64)
		log_info.Printf("converting `img` %v *image.RGBA64 into %T", imgFile.Name(), img)
	}

	if nrgba, ok := img.(*image.NRGBA); ok {
		img = nrgbaToRGBA(nrgba)
		log_info.Printf("converting `img` %v *image.NRGBA into %T", imgFile.Name(), img)
	}

	outputFile, err2 := os.Create(outputFilename)
	if err2 != nil {
		return err2
	}
	defer outputFile.Close()

	err3 := jpeg.Encode(outputFile, img, &jpeg.EncoderOptions{
		Quality:         *flag_g_jpg_quality,
		OptimizeCoding:  true,
		ProgressiveMode: *flag_g_progressive_jpeg,
	})
	if err3 != nil {
		return err3
	}

	return nil
}

/*
A little bot named Red lived in Paint Town. Red loved to help kids make their pictures look pretty!
One day, tiny Tim brought a clear sheet with colors on it. "I want to show my friends!" said Tim.
"I can help!" beeped Red. "Watch me make it pretty!"
Red took a white paper and looked at each tiny spot on Tim's clear sheet.
"When I see light spots," beeped Red, "I make them soft like clouds."
"When I see bright spots," beeped Red, "I make them strong like rainbows!"
Spot by spot, Red made Tim's picture. Some colors were soft, some were bright!
"Wow!" smiled Tim. "Now my friends can see my picture too!"
Red did a happy beep-boop dance. Helping make pictures was Red's favorite thing!
The End.
*/
func nrgbaToRGBA(src *image.NRGBA) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(b)

	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, b, a := src.At(x, y).RGBA()

			// Convert from non-premultiplied to premultiplied alpha
			r = (r * a) / 0xffff
			g = (g * a) / 0xffff
			b = (b * a) / 0xffff

			dst.SetRGBA(x, y, color.RGBA{
				R: uint8(r >> 8),
				G: uint8(g >> 8),
				B: uint8(b >> 8),
				A: uint8(a >> 8),
			})
		}
	}

	return dst
}

func palettedToRGBA(src *image.Paletted) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(b)

	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dst.Set(x, y, src.At(x, y))
		}
	}

	return dst
}

func palettedToYCbCr(src *image.Paletted) *image.YCbCr {
	b := src.Bounds()
	dst := image.NewYCbCr(b, image.YCbCrSubsampleRatio444)

	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, b, _ := src.At(x, y).RGBA()

			yy, cb, cr := color.RGBToYCbCr(uint8(r), uint8(g), uint8(b))

			i := dst.YOffset(x, y)
			dst.Y[i] = yy
			dst.Cb[i] = cb
			dst.Cr[i] = cr
		}
	}

	return dst
}

func rgba64ToRGBA(src *image.RGBA64) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(b)

	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r16, g16, b16, a16 := src.At(x, y).RGBA()

			r := uint8(r16 >> 8)
			g := uint8(g16 >> 8)
			b := uint8(b16 >> 8)
			a := uint8(a16 >> 8)

			dst.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: a})
		}
	}

	return dst
}

func validatePNGFile(file *os.File) error {
	var (
		err          error
		stat         os.FileInfo
		signature          = make([]byte, 8)
		lengthBuf          = make([]byte, 4)
		chunkTypeBuf       = make([]byte, 4)
		offset       int64 = 8 // Start after signature
		hasIHDR            = false
		hasIDAT            = false
		hasIEND            = false
	)

	if stat, err = file.Stat(); err != nil {
		return fmt.Errorf("failed to stat file: %v", err)
	}

	// Check min file size
	if stat.Size() < 33 {
		return fmt.Errorf("file too small to be valid PNG: %d bytes", stat.Size())
	}

	if _, err = file.ReadAt(signature, 0); err != nil {
		return fmt.Errorf("failed to read PNG signature: %v", err)
	}
	if !bytes.Equal(signature, []byte{137, 80, 78, 71, 13, 10, 26, 10}) {
		return fmt.Errorf("invalid PNG signature")
	}

	// Validate chunk structure
	for offset < stat.Size() {
		// Read chunk length
		if _, err = file.ReadAt(lengthBuf, offset); err != nil {
			return fmt.Errorf("failed to read chunk length at offset %d: %v", offset, err)
		}
		chunkLen := binary.BigEndian.Uint32(lengthBuf)

		// Read chunk type
		if _, err = file.ReadAt(chunkTypeBuf, offset+4); err != nil {
			return fmt.Errorf("failed to read chunk type at offset %d: %v", offset+4, err)
		}
		chunkType := string(chunkTypeBuf)

		// Validate chunk length
		if offset+int64(chunkLen)+12 > stat.Size() {
			return fmt.Errorf("chunk length %d at offset %d exceeds file size", chunkLen, offset)
		}

		// Track critical chunks
		switch chunkType {
		case "IHDR":
			if hasIHDR {
				return fmt.Errorf("multiple IHDR chunks found")
			}
			hasIHDR = true
			if chunkLen != 13 {
				return fmt.Errorf("invalid IHDR chunk length: %d", chunkLen)
			}
		case "IDAT":
			hasIDAT = true
		case "IEND":
			hasIEND = true
			if chunkLen != 0 {
				return fmt.Errorf("invalid IEND chunk length: %d", chunkLen)
			}
		}

		// Move to next chunk
		offset += int64(chunkLen) + 12 // length(4) + type(4) + data(chunkLen) + crc(4)
	}

	// Verify all critical chunks are present
	if !hasIHDR {
		return fmt.Errorf("missing IHDR chunk")
	}
	if !hasIDAT {
		return fmt.Errorf("missing IDAT chunk")
	}
	if !hasIEND {
		return fmt.Errorf("missing IEND chunk")
	}

	if _, err = file.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to reset file pointer: %v", err)
	}

	return nil
}

func overlayImages(jpgFile, pngFile *os.File, outputFilename string) error {
	sema_watermark.Acquire()
	defer sema_watermark.Release()
	jpgFile.Seek(0, 0)
	baseImg, _, err := image.Decode(jpgFile)
	if err != nil {
		return err
	}
	pngFile.Seek(0, 0)
	overlayImg, _, err := image.Decode(pngFile)
	if err != nil {
		return err
	}
	offset := image.Pt(0, 0)
	b := baseImg.Bounds()
	m := image.NewRGBA(b)
	draw.Draw(m, b, baseImg, image.Point{}, draw.Src)
	draw.Draw(m, overlayImg.Bounds().Add(offset), overlayImg, image.Point{}, draw.Over)
	outputFile, err := os.Create(outputFilename)
	if err != nil {
		return err
	}
	defer outputFile.Close()
	err = jpeg.Encode(outputFile, m, &jpeg.EncoderOptions{
		Quality:         *flag_g_jpg_quality,
		OptimizeCoding:  true,
		ProgressiveMode: *flag_g_progressive_jpeg,
	})
	if err != nil {
		return err
	}
	return nil
}

// colorDistance is responsible for calculating the Euclidean distance of the input colors and returns a since uint32.
// Euclidean distance is a measure of the straight-line distance between two points in Euclidean space. In other words,
// it's the distance between two points in a 2D or 3D plane. The Euclidean distance between two points is calculated
// by taking the square root of the sum of the squares of the differences between the corresponding coordinates of the
// two points.
// The formula for the Euclidean distance between two points, (x1,y1) and (x2,y2), in a 2D plane is:
//
//	distance = √((x2-x1)² + (y2-y1)²)
//
// The Euclidean distance is named after the ancient Greek mathematician Euclid, who is known for his work on geometry.
// Euclidean geometry deals with the properties of Euclidean space, which is a 2D or 3D space with a fixed distance
// metric. The concept of Euclidean distance is fundamental to Euclidean geometry.
// The Euclidean distance is widely used in various fields of science and engineering, including machine learning,
// computer vision, and physics. It is often used as a similarity measure between two vectors or data points in machine
// learning algorithms, such as k-nearest neighbors (KNN), support vector machines (SVM), and principal component
// analysis (PCA). In computer vision, Euclidean distance is used to calculate the distance between two pixels in
// an image.
// The Euclidean distance is also used in physics to calculate the distance between two points in space. For example,
// the distance between two stars in a galaxy can be calculated using the Euclidean distance formula.
// Overall, the Euclidean distance is a fundamental concept in mathematics and has wide applications in various fields.
// It is used to calculate distances between two points in a 2D or 3D space, and it is a key component of many machine
// learning algorithms and computer vision applications.
func colorDistance(c1, c2 color.Color) uint64 {
	r1, g1, b1, _ := c1.RGBA()
	r2, g2, b2, _ := c2.RGBA()

	dr := r1 - r2
	dg := g1 - g2
	db := b1 - b2

	return uint64(dr*dr + dg*dg + db*db)
}

func ConvertToDarkMode(img *os.File, directory, outputFilename string) (*os.File, error) {
	sem_darkimage.Acquire()
	defer sem_darkimage.Release()
	img.Seek(0, 0)
	srcImage, _, err := image.Decode(img)
	if err != nil {
		return img, err
	}
	dstImage := image.NewRGBA(srcImage.Bounds())
	// Iterate over all pixels in the image and apply the dark mode colors
	for y := dstImage.Bounds().Min.Y; y < dstImage.Bounds().Max.Y; y++ {
		for x := dstImage.Bounds().Min.X; x < dstImage.Bounds().Max.X; x++ {
			srcPixel := srcImage.At(x, y)

			if colorDistance(srcPixel, color.Black) <= uint64(0x050505)*uint64(0x050505) {
				dstImage.Set(x, y, color_text)
			} else if colorDistance(srcPixel, color.White) <= uint64(0x0F0F0F)*uint64(0x0F0F0F) {
				dstImage.Set(x, y, color_background)
			} else {
				dstImage.Set(x, y, srcPixel)
			}

		}
	}
	tempFile, err := os.CreateTemp(directory, outputFilename)
	if err != nil {
		return img, err
	}
	err = jpeg.Encode(tempFile, dstImage, &jpeg.EncoderOptions{
		Quality:         jpeg_compression_ratio,
		OptimizeCoding:  true,
		ProgressiveMode: true,
	})
	if err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name()) // Delete the temp file in case of error
		return img, err
	}
	tempFile.Seek(0, 0)
	return tempFile, nil
}

func verifyBinaries(binaries []string) error {
	for _, binary := range binaries {
		if runtime.GOOS == "windows" {
			binary += ".exe"
		}

		path, err := exec.LookPath(binary)
		if err != nil {
			return fmt.Errorf("binary '%s' not found in PATH", binary)
		}

		err = checkIfExecutable(path)
		if err != nil {
			return fmt.Errorf("binary '%s' is not executable: %w", binary, err)
		}

		m_required_binaries[binary] = path

		log.Printf("binary '%s' exists and is executable at path: %v", binary, path)
	}

	return nil
}

func DirHasPDFs(dirname string) (bool, error) {
	f, err := os.Open(dirname)
	if err != nil {
		return false, err
	}
	defer f.Close()

	files, err := f.Readdir(-1)
	if err != nil {
		return false, err
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".pdf") {
			return true, nil
		}
	}

	return false, nil
}

func checkIfExecutable(path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("binary does not exist")
	}

	if runtime.GOOS != "windows" && info.Mode()&c_dir_permissions == 0 {
		return fmt.Errorf("binary is not executable")
	}

	return nil
}

func NewIdentifier(length int) string {
	for {
		identifier := make([]byte, length)
		for i := range identifier {
			max := big.NewInt(int64(len(c_identifier_charset)))
			randIndex, err := rand.Int(rand.Reader, max)
			if err != nil {
				log_error.Tracef("failed to generate random number: %v", err)
				continue
			}
			identifier[i] = c_identifier_charset[randIndex.Int64()]
		}

		id := fmt.Sprintf("%4d%v", time.Now().UTC().Year(), string(identifier))

		mu_identifier.RLock()
		_, exists := m_used_identifiers[id]
		mu_identifier.RUnlock()

		if !exists {
			mu_identifier.Lock()
			m_used_identifiers[id] = true
			mu_identifier.Unlock()
			return id
		}
	}
}

func WritePendingPageToJson(pp PendingPage) error {
	sem_wjsonfile.Acquire()
	defer sem_wjsonfile.Release()

	file, err := os.OpenFile(pp.ManifestPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "    ")

	if err := encoder.Encode(pp); err != nil {
		return err
	}
	return nil
}

func WriteResultDataToJson(rd ResultData) error {
	sem_wjsonfile.Acquire()
	defer sem_wjsonfile.Release()
	file, err := os.OpenFile(rd.RecordPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "    ")

	if err := encoder.Encode(rd); err != nil {
		return err
	}

	return nil
}
