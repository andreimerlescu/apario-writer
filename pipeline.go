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
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func validate_result_data_record(ctx context.Context, record ResultData) (ResultData, error) {
	log_info.Printf("started validate_result_data_record(%v) = %v", record.Identifier, record.PDFPath)
	// analyze, repair on error, then re-analyze if necessary
	pdf_info, analyze_err := analyze_then_repair_pdf(record.PDFPath)
	if analyze_err != nil {
		return record, log_error.TraceReturn(analyze_err)
	}
	// fix total pages
	if pdf_info.Infos != nil && pdf_info.Infos[0].Pages == 0 && pdf_info.Infos[0].Pages != pdf_info.Infos[0].PageCount {
		pdf_info.Infos[0].Pages = pdf_info.Infos[0].PageCount
	}
	// validate total pages
	if pdf_info.Infos[0].Pages == 0 {
		return record, log_error.TraceReturnf("failed to set pdf_info.Pages to pdf_info.PageCount\npdf_info = %+v", pdf_info)
	}
	// validate pdf
	validate_err := validate_pdf(record.PDFPath)
	if validate_err != nil {
		return record, log_error.TraceReturn(validate_err)
	}
	// optimize pdf
	optimize_err := optimize_pdf(record.PDFPath)
	if optimize_err != nil {
		return record, log_error.TraceReturn(optimize_err)
	}
	return record, nil
}

func extractPlainTextFromPdf(ctx context.Context, record ResultData) {
	defer func() {
		log_info.Printf("finished extracting the text from the PDF %v, now sending rd into ch_ExtractPages", filepath.Base(record.PDFPath))
		if ch_ExtractPages.CanWrite() {
			err := ch_ExtractPages.Write(record)
			if err != nil {
				log_error.Tracef("failed to write record %v into the ch_ExtractPages due to error %v", record, err)
			}
		}
	}()
	log_info.Printf("started extractPlainTextFromPdf(%v) = %v", record.Identifier, record.PDFPath)
	if ok, err := fileHasData(record.ExtractedTextPath); !ok || err != nil {
		/*
			pdftotext REPLACE_WITH_FILE_PATH REPLACE_WITH_TEXT_OUTPUT_FILE_PATH
		*/
		cmd_extract_text_pdf := exec.Command(m_required_binaries["pdftotext"], record.PDFPath, record.ExtractedTextPath)
		var cmd4_extract_text_pdf_stdout bytes.Buffer
		var cmd4_extract_text_pdf_stderr bytes.Buffer
		cmd_extract_text_pdf.Stdout = &cmd4_extract_text_pdf_stdout
		cmd_extract_text_pdf.Stderr = &cmd4_extract_text_pdf_stderr
		sem_pdftotext.Acquire()
		cmd_extract_text_pdf_err := cmd_extract_text_pdf.Run()
		sem_pdftotext.Release()
		if cmd_extract_text_pdf_err != nil {
			log_error.Tracef("Failed to execute command `pdftotext %v %v` due to error: %s\n", record.PDFPath, record.ExtractedTextPath, cmd_extract_text_pdf_err)
			return
		}
	}
}

func extractPagesFromPdf(ctx context.Context, record ResultData) {
	log_info.Printf("started extractPagesFromPdf(%v) = %v", record.Identifier, record.PDFPath)
	/*
		pdfcpu extract -mode page REPLACE_WITH_FILE_PATH REPLACE_WITH_OUTPUT_DIRECTORY
	*/
	pagesDir := filepath.Join(record.DataDir, "pages")
	sm_page_directories.Store(record.Identifier, pagesDir)
	_, pagesDirExistsErr := os.Stat(pagesDir)
	performPagesExtract := false
	if os.IsNotExist(pagesDirExistsErr) {
		performPagesExtract = true
	} else {
		ok, err := DirHasPDFs(pagesDir)
		if err == nil && ok {
			performPagesExtract = true
		}
	}
RETRY:
	if performPagesExtract {
		pagesDirErr := os.MkdirAll(pagesDir, 0755)
		if pagesDirErr != nil {
			log_error.Tracef("failed to create directory %v due to error %v", pagesDir, pagesDirErr)
			return
		}
		cmd_extract_pages_in_pdf := exec.Command(m_required_binaries["pdfcpu"], "extract", "-mode", "page", record.PDFPath, pagesDir)
		var cmd_extract_pages_in_pdf_stdout bytes.Buffer
		var cmd_extract_pages_in_pdf_stderr bytes.Buffer
		cmd_extract_pages_in_pdf.Stdout = &cmd_extract_pages_in_pdf_stdout
		cmd_extract_pages_in_pdf.Stderr = &cmd_extract_pages_in_pdf_stderr
		sem_pdfcpu.Acquire()
		cmd_extract_pages_in_pdf_err := cmd_extract_pages_in_pdf.Run()
		sem_pdfcpu.Release()
		if cmd_extract_pages_in_pdf_err != nil {
			log_error.Tracef("Failed to execute command `pdfcpu extract -mode page %v %v` due to error: %s\n", record.PDFPath, pagesDir, cmd_extract_pages_in_pdf_err)
			return
		}
	} else {
		log_info.Printf("not performing `pdfcpu extrace -mode page %v %v` because the directory %v already has PDFs inside it", record.PDFPath, pagesDir, pagesDir)
		check := len(pagesDir) == len(pagesDir)

		if check {
			goto RETRY
		}
	}

	pagesDirWalkErr := filepath.Walk(pagesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log_error.Tracef("Error accessing a path %q: %v\n", path, err)
			return err
		}

		if !info.IsDir() && strings.HasSuffix(info.Name(), ".pdf") {
			nameParts := strings.Split(info.Name(), "_page_")
			if len(nameParts) < 2 {
				return fmt.Errorf("incorrect filename provided as %v", info.Name())
			}
			pgNoStr := strings.ReplaceAll(nameParts[1], ".pdf", "")
			pgNo, pgNoErr := strconv.Atoi(pgNoStr)
			if pgNoErr != nil {
				return fmt.Errorf("failed to extract the pgNo from the PDF filename %v", info.Name())
			}
			identifier := NewIdentifier(9)
			pp := PendingPage{
				Identifier:       identifier,
				RecordIdentifier: record.Identifier,
				PageNumber:       pgNo,
				PagesDir:         pagesDir,
				PDFPath:          path,
				OCRTextPath:      filepath.Join(pagesDir, fmt.Sprintf("ocr.%06d.txt", pgNo)),
				ManifestPath:     filepath.Join(pagesDir, fmt.Sprintf("page.%06d.json", pgNo)),
				PNG: PNG{
					Light: Images{
						Original: filepath.Join(pagesDir, fmt.Sprintf("page.light.%06d.original.png", pgNo)),
						Large:    filepath.Join(pagesDir, fmt.Sprintf("page.light.%06d.large.png", pgNo)),
						Medium:   filepath.Join(pagesDir, fmt.Sprintf("page.light.%06d.medium.png", pgNo)),
						Small:    filepath.Join(pagesDir, fmt.Sprintf("page.light.%06d.small.png", pgNo)),
						Social:   filepath.Join(pagesDir, fmt.Sprintf("page.light.%06d.social.png", pgNo)),
					},
					Dark: Images{
						Original: filepath.Join(pagesDir, fmt.Sprintf("page.dark.%06d.original.png", pgNo)),
						Large:    filepath.Join(pagesDir, fmt.Sprintf("page.dark.%06d.large.png", pgNo)),
						Medium:   filepath.Join(pagesDir, fmt.Sprintf("page.dark.%06d.medium.png", pgNo)),
						Small:    filepath.Join(pagesDir, fmt.Sprintf("page.dark.%06d.small.png", pgNo)),
						Social:   filepath.Join(pagesDir, fmt.Sprintf("page.dark.%06d.social.png", pgNo)),
					},
				},
				JPEG: JPEG{
					Light: Images{
						Original: filepath.Join(pagesDir, fmt.Sprintf("page.light.%06d.original.jpg", pgNo)),
						Large:    filepath.Join(pagesDir, fmt.Sprintf("page.light.%06d.large.jpg", pgNo)),
						Medium:   filepath.Join(pagesDir, fmt.Sprintf("page.light.%06d.medium.jpg", pgNo)),
						Small:    filepath.Join(pagesDir, fmt.Sprintf("page.light.%06d.small.jpg", pgNo)),
						Social:   filepath.Join(pagesDir, fmt.Sprintf("page.light.%06d.social.jpg", pgNo)),
					},
					Dark: Images{
						Original: filepath.Join(pagesDir, fmt.Sprintf("page.dark.%06d.original.jpg", pgNo)),
						Large:    filepath.Join(pagesDir, fmt.Sprintf("page.dark.%06d.large.jpg", pgNo)),
						Medium:   filepath.Join(pagesDir, fmt.Sprintf("page.dark.%06d.medium.jpg", pgNo)),
						Small:    filepath.Join(pagesDir, fmt.Sprintf("page.dark.%06d.small.jpg", pgNo)),
						Social:   filepath.Join(pagesDir, fmt.Sprintf("page.dark.%06d.social.jpg", pgNo)),
					},
				},
			}
			sm_pages.Store(pp.Identifier, pp)
			err := WritePendingPageToJson(pp)
			if err != nil {
				return err
			}
			log_info.Printf("sending page %d (ID %v) from record %v URL %v into the ch_GeneratingPng", pgNo, identifier, record.Identifier, record.URL)
			if ch_GeneratePng.CanWrite() {
				err := ch_GeneratePng.Write(pp)
				if err != nil {
					log_error.Tracef("cannot send pp into ch_GeneratePng channel due to error %v", err)
					return err
				}
			}
		}

		return nil
	})

	if pagesDirWalkErr != nil {
		log_error.Tracef("Error walking the path ./pages: %v\n", pagesDirWalkErr)
		return
	}

	return
}

func convertPageToPng(ctx context.Context, pp PendingPage) {
	log_info.Printf("started convertPageToPng(%v.%v) = %v", pp.RecordIdentifier, pp.Identifier, pp.PDFPath)
	/*
		pdf_to_png: "pdftoppm REPLACE_WITH_PNG_OPTS REPLACE_WITH_FILE_PATH REPLACE_WITH_PNG_PATH",
	*/
RECHECK:
	_, loErr := os.Stat(pp.PNG.Light.Original)
	if os.IsNotExist(loErr) {
		originalFilename := strings.ReplaceAll(pp.PNG.Light.Original, `.png`, ``)
		cmd := exec.Command(m_required_binaries["pdftoppm"],
			`-r`, `369`, `-png`, `-freetype`, `yes`, `-aa`, `yes`, `-aaVector`, `yes`, `-thinlinemode`, `solid`,
			pp.PDFPath, originalFilename)
		var cmd_stdout bytes.Buffer
		var cmd_stderr bytes.Buffer
		cmd.Stdout = &cmd_stdout
		cmd.Stderr = &cmd_stderr
		sem_pdftoppm.Acquire()
		cmd_err := cmd.Run()
		sem_pdftoppm.Release()
		if cmd_err != nil {
			log_error.Tracef("failed to convert page %v to png %v due to error: %s\n", filepath.Base(pp.PDFPath), pp.PNG.Light.Original, cmd_err)
			return
		}

		pngRenameErr := os.Rename(fmt.Sprintf("%v-1.png", originalFilename), fmt.Sprintf("%v.png", originalFilename))
		if pngRenameErr != nil {
			log_error.Tracef("failed to rename the jpg %v due to error: %v", originalFilename, pngRenameErr)
			return
		}
	} else {
		originalFile, fileErr := os.Open(pp.PNG.Light.Original)
		if fileErr != nil {
			if err1 := validatePNGFile(originalFile); err1 != nil {
				if err2 := os.Remove(pp.PNG.Light.Original); err2 != nil {
					goto RECHECK
				} else {
					msg := "convertPagePng() pp.PNG.Light.Original exists and has thrown 2 errors:\n" +
						"err1 [validatePNGFile(originalFile): %+v\n" +
						"err2 [os.Remove(pp.PNG.Light.Original]: %+v\n"
					log_error.Panicf(msg, err1, err2)
				}
			}

		}
	}

	log_info.Printf("completed convertPageToPng now sending %v (%v.%v) -> ch_GenerateLight ", pp.PDFPath, pp.RecordIdentifier, pp.Identifier)
	if ch_GenerateLight.CanWrite() {
		err := ch_GenerateLight.Write(pp)
		if err != nil {
			log_error.Tracef("canot send pp into ch_GenerateLight due to error %v", err)
			return
		}
	}
	return
}

func generateLightThumbnails(ctx context.Context, pp PendingPage) {
	defer func() {
		log_info.Printf("completed generateLightThumbnails now sending %v (%v.%v) -> ch_GenerateDark ", pp.PDFPath, pp.RecordIdentifier, pp.Identifier)
		if ch_GenerateDark.CanWrite() {
			err := ch_GenerateDark.Write(pp)
			if err != nil {
				log_error.Tracef("cannot send pp into the ch_GenerateDark due to error %v", err)
				return
			}
		}
	}()
	log_info.Printf("started generateLightThumbnails(%v.%v) = %v", pp.RecordIdentifier, pp.Identifier, pp.PDFPath)

	original, err := os.Open(pp.PNG.Light.Original)
	if err != nil {
		log_error.Tracef("failed to open pp.OriginalPath(%v) due to error %v", pp.PNG.Light.Original, err)
		return
	}

	// create the large thumbnail from the JPG
	_, llgErr := os.Stat(pp.PNG.Light.Large)
	if os.IsNotExist(llgErr) {
		lgResizeErr := resizePng(original, 999, pp.PNG.Light.Large)
		if lgResizeErr != nil {
			log_error.Tracef("failed to resize jpg %v due to error %v", pp.PNG.Light.Large, lgResizeErr)
			return
		}
	}

	// create the medium thumbnail from the JPG
	_, lmdErr := os.Stat(pp.PNG.Light.Medium)
	if os.IsNotExist(lmdErr) {
		mdResizeErr := resizePng(original, 666, pp.PNG.Light.Medium)
		if mdResizeErr != nil {
			log_error.Tracef("failed to resize jpg %v due to error %v", pp.PNG.Light.Medium, mdResizeErr)
			return
		}
	}

	// create the small thumbnail from the JPG
	_, lsmErr := os.Stat(pp.PNG.Light.Small)
	if os.IsNotExist(lsmErr) {
		smResizeErr := resizePng(original, 333, pp.PNG.Light.Small)
		if smResizeErr != nil {
			log_error.Tracef("failed to resize jpg %v due to error %v", pp.PNG.Light.Small, smResizeErr)
			return
		}
	}

}

func generateDarkThumbnails(ctx context.Context, pp PendingPage) {
	defer func() {
		log_info.Printf("completed generateDarkThumbnails now sending %v (%v.%v) -> ch_PerformOcr ", pp.PDFPath, pp.RecordIdentifier, pp.Identifier)
		if ch_PerformOcr.CanWrite() {
			err := ch_PerformOcr.Write(pp)
			if err != nil {
				log_error.Tracef("cant write to the ch_PerformOcr due to error %v", err)
				return
			}
		}
	}()
	log_info.Printf("started generateDarkThumbnails(%v.%v) = %v", pp.RecordIdentifier, pp.Identifier, pp.PDFPath)
	// task: the pp.Light.Original into pp.Dark.Original

	_, ppdoErr := os.Stat(pp.PNG.Dark.Original)
	if os.IsNotExist(ppdoErr) {
		// convert REPLACE_WITH_OUTPUT_PNG_PAGE_FILENAME -channel rgba -matte -fill 'rgba(250,226,203,1)' -fuzz 45% -opaque 'rgba(76,76,76,1)' -flatten REPLACE_WITH_OUTPUT_PNG_DARK_PAGE_FILENAME
		cmdA := exec.Command(m_required_binaries["convert"], pp.PNG.Light.Original, "-channel", "rgba", "-matte", "-fill", `rgba(250,226,203,1)`, "-fuzz", "45%", "-opaque", `rgba(76,76,76,1)`, "-flatten", pp.PNG.Dark.Original)
		var cmdA_stdout bytes.Buffer
		var cmdA_stderr bytes.Buffer
		cmdA.Stdout = &cmdA_stdout
		cmdA.Stderr = &cmdA_stderr
		sem_convert.Acquire()
		cmdA_err := cmdA.Run()
		sem_convert.Release()
		if cmdA_err != nil {
			log_info.Tracef("failed to convert %v into %v due to error: %s\n", pp.PNG.Light.Original, pp.PNG.Dark.Original, cmdA_err)
			return
		}

		// convert REPLACE_WITH_OUTPUT_PNG_DARK_PAGE_FILENAME -channel rgba -matte -fill 'rgba(40,40,86,1)' -fuzz 12% -opaque white -flatten REPLACE_WITH_OUTPUT_PNG_DARK_PAGE_FILENAME
		cmdB := exec.Command(m_required_binaries["convert"], pp.PNG.Dark.Original, `-channel`, `rgba`, `-matte`, `-fill`, `rgba(40,40,86,1)`, `-fuzz`, `12%`, `-opaque`, `white`, `-flatten`, pp.PNG.Dark.Original)
		var cmdB_stdout bytes.Buffer
		var cmdB_stderr bytes.Buffer
		cmdB.Stdout = &cmdB_stdout
		cmdB.Stderr = &cmdB_stderr
		sem_convert.Acquire()
		cmdB_err := cmdB.Run()
		sem_convert.Release()
		if cmdB_err != nil {
			log_error.Tracef("failed to convert %v into %v due to error: %s\n", pp.PNG.Light.Original, pp.PNG.Dark.Original, cmdB_err)
			return
		}
	}

	original, err := os.Open(pp.PNG.Dark.Original)
	if err != nil {
		log_error.Tracef("failed to open pp.OriginalPath(%v) due to error %v", pp.PNG.Dark.Original, err)
		return
	}

	// create the large thumbnail from the JPG
	_, dlgErr := os.Stat(pp.PNG.Dark.Large)
	if os.IsNotExist(dlgErr) {
		lgResizeErr := resizePng(original, 999, pp.PNG.Dark.Large)
		if lgResizeErr != nil {
			log_error.Tracef("failed to resize jpg %v due to error %v", pp.PNG.Dark.Large, lgResizeErr)
			return
		}
	}

	// create the medium thumbnail from the JPG
	_, dmdErr := os.Stat(pp.PNG.Dark.Medium)
	if os.IsNotExist(dmdErr) {
		mdResizeErr := resizePng(original, 666, pp.PNG.Dark.Medium)
		if mdResizeErr != nil {
			log_error.Tracef("failed to resize jpg %v due to error %v", pp.PNG.Dark.Medium, mdResizeErr)
			return
		}
	}

	// create the small thumbnail from the JPG
	_, dsmErr := os.Stat(pp.PNG.Dark.Small)
	if os.IsNotExist(dsmErr) {
		smResizeErr := resizePng(original, 333, pp.PNG.Dark.Small)
		if smResizeErr != nil {
			log_error.Tracef("failed to resize jpg %v due to error %v", pp.PNG.Dark.Small, smResizeErr)
			return
		}
	}

}

func performOcrOnPdf(ctx context.Context, pp PendingPage) {
	defer func() {
		log_info.Printf("completed performOcrOnPdf now sending %v (%v.%v) -> ch_ConvertToJpg ", pp.PDFPath, pp.RecordIdentifier, pp.Identifier)
		if ch_ConvertToJpg.CanWrite() {
			err := ch_ConvertToJpg.Write(pp)
			if err != nil {
				log_error.Tracef("cant write to the ch_ConverToJpg due to error %v", err)
				return
			}
		}
	}()

	if ok, err := fileHasData(pp.OCRTextPath); !ok || err != nil {
		/*
			tesseract SRC DEST -l eng --psm 1
		*/
		ocrStat, ppOcrPathErr := os.Stat(pp.OCRTextPath)
		if (ppOcrPathErr == nil || !os.IsNotExist(ppOcrPathErr)) && ocrStat.Size() > 0 {
			ocrText, ocrTextErr := os.ReadFile(pp.OCRTextPath)
			if ocrTextErr != nil && len(string(ocrText)) > 6 {
				log_info.Printf(
					"finished performOcrOnPdf(%v.%v) because the file %v already has %d bytes inside it!",
					pp.RecordIdentifier, pp.Identifier, pp.OCRTextPath, ocrStat.Size())
				return
			}
		}
		src := pp.PNG.Light.Original
		dest := strings.TrimSuffix(pp.OCRTextPath, `.txt`)
		cmd := exec.Command(m_required_binaries["tesseract"], src, dest, `-l`, `eng`, `--psm`, `1`)
		var cmd_stdout bytes.Buffer
		var cmd_stderr bytes.Buffer
		cmd.Stdout = &cmd_stdout
		cmd.Stderr = &cmd_stderr
		log_info.Printf("started performOcrOnPdf(%v.%v) = %v (WAITING)", pp.RecordIdentifier, pp.Identifier, pp.PDFPath)
		sem_tesseract.Acquire()
		log_info.Printf("running performOcrOnPdf(%v.%v) = %v", pp.RecordIdentifier, pp.Identifier, pp.PDFPath)
		cmd_err := cmd.Run()
		sem_tesseract.Release()
		log_info.Printf("completed performOcrOnPdf(%v.%v) = %v", pp.RecordIdentifier, pp.Identifier, pp.PDFPath)
		if cmd_err != nil {
			log_error.Tracef(
				"Command `tesseract %v %v -l eng --psm 1` failed with error: %s\n\n\tSTDERR = %v\n\tSTDOUT = %v\n",
				src, dest, cmd_err, cmd_stderr.String(), cmd_stdout.String())
			return
		}
	}
}

func convertPngToJpg(ctx context.Context, pp PendingPage) {
	defer func() {
		log_info.Printf("completed convertPngToJpg now sending %v (%v.%v) -> ch_CompletedPage ", pp.PDFPath, pp.RecordIdentifier, pp.Identifier)
		if ch_AnalyzeText.CanWrite() {
			err := ch_AnalyzeText.Write(pp)
			if err != nil {
				log_error.Tracef("cant write to the ch_AnalyzeText channel due to error %v", err)
				return
			}
		}
	}()
	log_info.Printf("started convertPngToJpg(%v.%v) = %v", pp.RecordIdentifier, pp.Identifier, pp.PDFPath)
	files := map[string]string{
		pp.PNG.Light.Original: pp.JPEG.Light.Original,
		pp.PNG.Light.Large:    pp.JPEG.Light.Large,
		pp.PNG.Light.Medium:   pp.JPEG.Light.Medium,
		pp.PNG.Light.Small:    pp.JPEG.Light.Small,
		pp.PNG.Light.Social:   pp.JPEG.Light.Social,
		pp.PNG.Dark.Original:  pp.JPEG.Dark.Original,
		pp.PNG.Dark.Large:     pp.JPEG.Dark.Large,
		pp.PNG.Dark.Medium:    pp.JPEG.Dark.Medium,
		pp.PNG.Dark.Small:     pp.JPEG.Dark.Small,
		pp.PNG.Dark.Social:    pp.JPEG.Dark.Social,
	}
	for png, jpeg := range files {
		if strings.HasSuffix(png, `social.png`) || strings.HasSuffix(jpeg, `social.jpg`) {
			// TODO - skip social.png until its implemented
			continue
		}
		f, e1 := os.Open(png)
		if e1 != nil {
			log_error.Tracef("failed to convertAndOptimizePNG for file %v due to error %v", png, e1)
			continue
		}
		e2 := convertAndOptimizePNG(f, jpeg)
		if e2 != nil {
			log_error.Tracef("failed to convertAndOptimizePNG(%v) due to error %v", png, e2)
			continue
		}

		_, pngErr := os.Stat(png)
		if !os.IsNotExist(pngErr) {
			e3 := os.Remove(png)
			if e3 != nil {
				log_error.Tracef("failed to remove PNG file %v due to error %v", png, e3)
			}
		}
	}

}

// compileDarkPDF TODO: need to implement this so page.dark.######.original.jpg can be combined into <filename>.dark.pdf
func compileDarkPDF() {

}

// generateSocialCard TODO: need to implement creating the social image card for X/Facebook/etc. when links are shared
func generateSocialCard() {

}
