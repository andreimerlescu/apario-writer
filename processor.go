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
	"bufio"
	`bytes`
	"context"
	"encoding/csv"
	`encoding/json`
	`fmt`
	"log"
	"os"
	`os/exec`
	`path/filepath`
	"strings"
	"sync"
	"sync/atomic"

	"github.com/tealeg/xlsx"
)

func process_import_csv(ctx context.Context, filename string, callback CallbackFunc) error {
	file, openErr := os.Open(filename)
	if openErr != nil {
		log.Printf("cant open the file because of err: %v", openErr)
		return openErr
	}
	defer func(file *os.File) {
		closeErr := file.Close()
		if closeErr != nil {
			log.Fatalf("failed to close the file %v caused error %v", filename, closeErr)
		}
	}(file)
	bufferedReader := bufio.NewReaderSize(file, reader_buffer_bytes)
	reader := csv.NewReader(bufferedReader)
	if strings.HasSuffix(filename, ".psv") {
		reader.Comma = '|'
	}
	reader.FieldsPerRecord = -1
	headerFields, bufferReadErr := reader.Read()
	if bufferReadErr != nil {
		log.Printf("cant read the csv buffer because of err: %v", bufferReadErr)
		return bufferReadErr
	}
	log.Printf("headerFields = %v", strings.Join(headerFields, ","))
	row := make(chan []Column, channel_buffer_size)
	totalRows, rowWg := atomic.Uint32{}, sync.WaitGroup{}
	done := make(chan struct{})
	go ReceiveRows(ctx, row, filename, callback, done)
	for {
		rowFields, readerErr := reader.Read()
		if readerErr != nil {
			log.Printf("skipping row due to error %v with data %v", readerErr, rowFields)
			break
		}
		totalRows.Add(1)
		rowWg.Add(1)
		go ProcessRow(headerFields, rowFields, &rowWg, row)
	}

	rowWg.Wait()
	close(row)
	<-done
	log.Printf("totalRows = %d", totalRows.Load())
	return nil
}

func process_import_xlsx(ctx context.Context, filename string, callback CallbackFunc) error {
	file, err := xlsx.OpenFile(filename)
	if err != nil {
		log.Printf("cant open the file because of err: %v", err)
		return err
	}
	sheet := file.Sheets[0]
	headerFields := make([]string, 0, len(sheet.Rows[0].Cells))
	for _, cell := range sheet.Rows[0].Cells {
		if len(cell.String()) > 0 {
			headerFields = append(headerFields, cell.String())
		}
	}
	log.Printf("headerFields = %v", strings.Join(headerFields, ","))
	row := make(chan []Column, channel_buffer_size)
	totalRows, rowWg := atomic.Uint32{}, sync.WaitGroup{}
	done := make(chan struct{})
	go ReceiveRows(ctx, row, filename, callback, done)
	for _, sheetRow := range sheet.Rows[1:] {
		rowFields := make([]string, 0, len(sheetRow.Cells))
		for _, cell := range sheetRow.Cells {
			rowFields = append(rowFields, cell.String())
		}
		totalRows.Add(1)
		rowWg.Add(1)
		go ProcessRow(headerFields, rowFields, &rowWg, row)
	}
	rowWg.Wait()
	close(row)
	<-done
	log.Printf("totalRows = %d", totalRows.Load())
	return nil
}

func process_download_pdf(ctx context.Context, filename string) error {
	source_url := *flag_s_download_pdf_url

	if !strings.HasPrefix(source_url, "http") {
		if len(source_url) > 0 {
			// has a value, but it doesnt begin with http
			log.Printf("ERROR: --download-pdf-url doesn't begin with http but has a value of %v", source_url)
		}
	}

	pdf_url_checksum := Sha256(source_url)

	identifier := NewIdentifier(6)

	recordDir := filepath.Join(*flag_s_database_directory, pdf_url_checksum)
	err := os.MkdirAll(recordDir, 0750)
	if err != nil {
		return err
	}

	var (
		q_file_pdf       = filepath.Join(recordDir, strings.ReplaceAll(filename, `/`, `_`))
		q_file_ocr       = filepath.Join(recordDir, "ocr.txt")
		q_file_extracted = filepath.Join(recordDir, "extracted.txt")
		q_file_record    = filepath.Join(recordDir, "record.json")
	)

	_, downloadedPdfErr := os.Stat(q_file_pdf)
	if os.IsNotExist(downloadedPdfErr) {
		log.Printf("downloading URL %v to %v", source_url, q_file_pdf)
		err = downloadFile(ctx, source_url, q_file_pdf)
		if err != nil {
			return err
		}
	}

	// [-TO-DO-]: first the downloaded file must be scanned through a virus scanner, this will introduce a runtime requirement release process update
	// TODO: ensure clamav is installed via the release upgrade script
	output, action_taken, clamav_scan_err := scan_path_with_clam_av(q_file_pdf)
	if clamav_scan_err != nil {
		return clamav_scan_err
	}

	if action_taken {
		log.Printf("action taken against %v with clamav: %v", q_file_pdf, output)
		return fmt.Errorf("antivirus action taken against %v", q_file_pdf)
	}

	// [-TO-DO-]: analyze the metadata of the pdf file to determine totalPages, currently defaulting to 0
	pageCount, containsText, pdf_analysis_err := analyze_pdf_path(q_file_pdf)
	if pdf_analysis_err != nil {
		fmt.Println("Error:", err)
		return pdf_analysis_err
	}

	pdfFile, pdfFileErr := os.Open(q_file_pdf) // [-TO-DO-]: need to add some security around this process
	if pdfFileErr != nil {
		return pdfFileErr
	}
	checksum := FileSha512(pdfFile)
	pdfFile.Close()

	metadata := make(map[string]string)
	metadata_bytes := bytes.NewBufferString(*flag_s_pdf_metadata_json).Bytes()
	err = json.Unmarshal(metadata_bytes, &metadata)
	if err != nil {
		return err
	}

	var pdf_text string
	var pdf_text_err error
	if containsText {
		pdf_text, pdf_text_err = extract_text_from_pdf(source_url)
		if pdf_text_err != nil {
			log.Printf("pdf_text_err = %v", pdf_text_err)
		}

		if len(pdf_text) > 17 {
			save_extracted_err := write_string_to_file(q_file_extracted, pdf_text)
			if save_extracted_err != nil {
				log.Printf("save_extracted_err = %v", save_extracted_err)
			}
		}
	}
	pdf_text = "" // flush the data out, its not needed any longer

	rd := ResultData{
		Identifier:        identifier,
		URL:               source_url,
		DataDir:           recordDir,
		TotalPages:        int64(pageCount),
		PDFChecksum:       checksum,
		PDFPath:           q_file_pdf,
		OCRTextPath:       q_file_ocr,
		ExtractedTextPath: q_file_extracted,
		RecordPath:        q_file_record,
		Metadata:          metadata,
	}
	err = WriteResultDataToJson(rd)
	if err != nil {
		return err
	}
	sm_documents.Store(identifier, rd)
	log.Printf("sending URL %v (rd struct) into the ch_ImportedRow channel", rd.URL)
	err = ch_ImportedRow.Write(rd)
	if err != nil {
		log.Printf("cant write to ch_ImportedRow")
		return err
	}
	return nil
}

func process_import_pdf(ctx context.Context, filename string) error {
	// TODO: implement import pdf from local filesystem
	return nil
}

func process_import_directory(ctx context.Context, filename string) error {
	// TODO: implement import directory from local filesystem
	return nil
}

// scan_path_with_clam_av scans the specified path with ClamAV and returns the results.
func scan_path_with_clam_av(path string) (string, bool, error) {
	// Prepare the clamscan command
	cmd := exec.Command("clamscan", "--infected", "--remove", path)

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := out.String()
	if err != nil {
		// Return the error along with the stderr output from clamscan
		return output, false, fmt.Errorf("%v: %s", err, stderr.String())
	}

	// Check if an action was taken, like a file being removed.
	// Adjust the keywords as necessary based on your version of clamscan and its output.
	actionTaken := strings.Contains(output, "Removed") || strings.Contains(output, "FOUND")

	// Return the output from clamscan and whether an action was taken
	return output, actionTaken, nil
}

// write_string_to_file writes the provided string data to the specified file.
func write_string_to_file(filename, data string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(data)
	if err != nil {
		return err
	}
	err = file.Sync()
	if err != nil {
		return err
	}
	return nil
}

// extract_text_from_pdf uses the `pdftotext` utility to extract text from a PDF file.
func extract_text_from_pdf(path string) (string, error) {
	// -layout flag is optional, it helps in maintaining the original physical layout of the text.
	cmd := exec.Command("pdftotext", "-layout", path, "-")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return out.String(), nil
}

// analyze_pdf_path uses the `pdfcpu` utility to determine properties about a PDF file.
func analyze_pdf_path(path string) (int, bool, error) {
	cmd := exec.Command("pdfcpu", "validate", "-verbose", path)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return 0, false, err
	}
	output := out.String()
	pageCount := 0
	containsText := false
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "pages:") {
			// Extract page count from the line
			_, err := fmt.Sscanf(line, "pages: %d", &pageCount)
			if err != nil {
				return 0, false, err
			}
		}
		if strings.Contains(line, "fonts:") {
			containsText = true
		}
	}

	return pageCount, containsText, nil
}

func ProcessRow(headerFields []string, rowFields []string, rowWg *sync.WaitGroup, row chan []Column) {
	defer rowWg.Done()
	var d = map[string]string{}
	if len(headerFields) != len(rowFields) {
		if len(headerFields) < len(rowFields) {
			for i, r := range rowFields {
				if i >= len(headerFields) || len(r) == 0 {
					continue
				}
				d[headerFields[i]] = r
			}
		} else {
			for i, h := range headerFields {
				if i >= len(rowFields) || len(h) == 0 {
					continue
				}
				d[h] = rowFields[i]
			}
		}
	}
	var rowData = []Column{}
	if len(d) > 0 {
		for h, v := range d {
			rowData = append(rowData, Column{Header: h, Value: v})
		}
	} else {
		for i := 0; i < len(rowFields); i++ {
			value := rowFields[i]
			if i == 0 && len(value) == 0 {
				return
			}
			if len(headerFields) < i {
				log.Printf("skipping rowField %v due to headerFields not matching up properly", rowFields[i])
				continue
			}
			rowData = append(rowData, Column{headerFields[i], value})
		}
	}
	row <- rowData
}

func ReceiveRows(ctx context.Context, row chan []Column, filename string, callback CallbackFunc, done chan struct{}) {
	for {
		select {
		case <-ctx.Done():
			return
		case populatedRow, ok := <-row:
			if !ok {
				done <- struct{}{}
				return
			}
			ctx := context.WithValue(ctx, CtxKey("csv_file"), filename)
			callbackErr := callback(ctx, populatedRow)
			if callbackErr != nil {
				log.Printf("failed to insert row %v with error %v", populatedRow, callbackErr)
			}
		}
	}
}
