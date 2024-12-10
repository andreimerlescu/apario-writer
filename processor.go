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
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	countable_waitgroup "github.com/andreimerlescu/go-countable-waitgroup"
	"github.com/tealeg/xlsx"
)

func process_import_csv(ctx context.Context, filename string, callback CallbackFunc) error {
	file, openErr := os.Open(filename)
	if openErr != nil {
		log_error.Tracef("cant open the file because of err: %v", openErr)
		return openErr
	}
	defer func(file *os.File) {
		closeErr := file.Close()
		if closeErr != nil {
			log_error.Fatalf("failed to close the file %v caused error %v", filename, closeErr)
		}
	}(file)
	bufferedReader := bufio.NewReaderSize(file, reader_buffer_bytes)
	reader := csv.NewReader(bufferedReader)
	if strings.HasSuffix(filename, ".psv") {
		reader.Comma = '|'
	}
	if strings.HasSuffix(filename, ".tsv") {
		reader.Comma = '	'
	}
	reader.FieldsPerRecord = -1
	headerFields, bufferReadErr := reader.Read()
	if bufferReadErr != nil {
		log_error.Tracef("cant read the csv buffer because of err: %v", bufferReadErr)
		return bufferReadErr
	}
	log_info.Printf("headerFields = %v", strings.Join(headerFields, ","))
	row := make(chan []Column, channel_buffer_size)
	totalRows, rowWg := atomic.Uint32{}, sync.WaitGroup{}
	done := make(chan struct{})
	go ReceiveRows(ctx, row, filename, callback, done)
	for {
		rowFields, readerErr := reader.Read()
		if readerErr != nil {
			log_info.Printf("skipping row due to error %v with data %v", readerErr, rowFields)
			break
		}
		totalRows.Add(1)
		rowWg.Add(1)
		go ProcessRow(headerFields, rowFields, &rowWg, row)
	}

	rowWg.Wait()
	close(row)
	<-done
	log_info.Printf("totalRows = %d", totalRows.Load())
	return nil
}

func process_import_xlsx(ctx context.Context, filename string, callback CallbackFunc) error {
	file, err := xlsx.OpenFile(filename)
	if err != nil {
		log_error.Tracef("cant open the file because of err: %v", err)
		return err
	}
	sheet := file.Sheets[0]
	headerFields := make([]string, 0, len(sheet.Rows[0].Cells))
	for _, cell := range sheet.Rows[0].Cells {
		if len(cell.String()) > 0 {
			headerFields = append(headerFields, cell.String())
		}
	}
	log_info.Printf("headerFields = %v", strings.Join(headerFields, ","))
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
	log_info.Printf("totalRows = %d", totalRows.Load())
	return nil
}

func process_download_pdf(ctx context.Context, source_url string, metadata_json string) error {
	url_source, url_err := url.Parse(source_url)
	if url_err != nil {
		return url_err
	}
	filename := filepath.Base(url_source.Path)
	log_info.Printf("process_download_pdf(%v) has a filename of %v", source_url, filename)
	if url_source.Scheme != "https" {
		if len(source_url) > 0 {
			// has a value, but it doesnt begin with http
			log_error.Tracef("invalid source_url provided %v", source_url)
			return fmt.Errorf("ERROR: --download-pdf-url doesn't begin with http but has a value of %v", source_url)
		}
	}

	pdf_url_checksum := Sha256(source_url)

	identifier := NewIdentifier(6)

	recordDir := filepath.Join(*flag_s_database_directory, pdf_url_checksum)
	err := os.MkdirAll(recordDir, 0750)
	if err != nil {
		log_error.Tracef("cannot mkdir -p %v due to err %v", recordDir, err)
		return err
	}

	basename := filepath.Base(filename)
	var (
		q_file_pdf       = filepath.Join(recordDir, strings.ReplaceAll(basename, `/`, `_`))
		q_file_ocr       = filepath.Join(recordDir, "ocr.txt")
		q_file_extracted = filepath.Join(recordDir, "extracted.txt")
		q_file_record    = filepath.Join(recordDir, "record.json")
	)

	_, downloadedPdfErr := os.Stat(q_file_pdf)
	if os.IsNotExist(downloadedPdfErr) {
		log_info.Printf("downloading URL %v to %v", source_url, q_file_pdf)
		err = downloadFile(ctx, source_url, q_file_pdf)
		if err != nil {
			log_error.Tracef("received an error while downloading the file")
			return err
		}
	}

	// [-TO-DO-]: first the downloaded file must be scanned through a virus scanner, this will introduce a runtime requirement release process update
	// TODO: ensure clamav is installed via the release upgrade script
	if !*flag_b_disable_clamav {
		output, action_taken, clamav_scan_err := scan_path_with_clam_av(q_file_pdf)
		if clamav_scan_err != nil {
			log_error.Tracef("while scanning %v clamav scan returned an err: %v", q_file_pdf, clamav_scan_err)
			return clamav_scan_err
		}

		if action_taken {
			log_debug.Tracef("action taken against %v with clamav: %v", q_file_pdf, output)
			return fmt.Errorf("antivirus action taken against %v", q_file_pdf)
		}
	}

	// [-TO-DO-]: analyze the metadata of the pdf file to determine totalPages, currently defaulting to 0
	pdf_analysis, pdf_analysis_err := analyze_pdf_path(q_file_pdf)
	if pdf_analysis_err != nil {
		/*
			this double check and overload on pdf_analysis, pdf_analysis_err is due to a bug found in the apario-writer
			while ingesting the Project Minnesota Election Selections documents. How PDF files are rendered matters to
			open source software, and proprietary PDF software is notorious for putting corrupted data inside the files
			that render without issue on their programs, but on open source libraries like that which apario-reader uses
			the error shows up on linux systems and as such; we were getting to the end of the collection in a few hours
			but the program was never exiting; this was due to the mix-matching count of a_i_documents to write to ch_Done
			but, given that there were errors in the PDFs analysis, the metrics about getting the PDF file was successful,
			but downstream errors in the same PDF files were rooted in the problem here as the PDFCPUInfoResponse struct
			is corrupted. All good!
		*/
		pdf_analysis, pdf_analysis_err = repair_then_analyze_pdf(q_file_pdf)
		if pdf_analysis_err != nil {
			log_error.Tracef("received an err %v on pdf_analysis [187] for %v", err, q_file_pdf)
			return pdf_analysis_err
		}
	}

	var info *PDFCPUInfoResponseInfo
	if len(pdf_analysis.Infos) > 0 {
		data := pdf_analysis.Infos[0] // capture
		info = &data                  // point
	}

	if info == nil {
		log_debug.Tracef("WARN info is nil in process_download_pdf")
		info = &PDFCPUInfoResponseInfo{}
	}

	var embedded_text string
	if info != nil {
		if info.Pages > 0 {
			a_i_total_pages.Add(int64(info.Pages))
		}
		if len(info.Keywords) > 0 {
			embedded_text = strings.Join(info.Keywords, " ")
			embedded_text = strings.ReplaceAll(embedded_text, "  ", " ")
		}
	}

	pdfFile, pdfFileErr := os.Open(q_file_pdf) // [-TO-DO-]: need to add some security around this process
	if pdfFileErr != nil {
		return pdfFileErr
	}
	checksum := FileSha512(pdfFile)
	pdf_close_err := pdfFile.Close()
	if pdf_close_err != nil {
		return pdf_close_err
	}

	metadata := make(map[string]string)
	if len(metadata_json) > 0 {
		metadata_bytes := bytes.NewBufferString(metadata_json).Bytes()
		err = json.Unmarshal(metadata_bytes, &metadata)
		metadata_bytes = nil
		if err != nil {
			log_error.Tracef("failed to parse the --metadata-json due to err %v", err)
		}
	}

	pdf_text, pdf_text_err := extract_text_from_pdf(q_file_pdf)
	if pdf_text_err != nil {
		log_error.Tracef("pdf_text_err = %v", pdf_text_err)
	}

	log_info.Printf("comparing pdf_text to embedded_text")

	if len(embedded_text) > 17 {
		save_extracted_err := write_string_to_file(q_file_extracted, embedded_text)
		if save_extracted_err != nil {
			log_error.Tracef("save_extracted_err = %v", save_extracted_err)
		}
		info.Keywords = []string{} // flush memory
	} else if len(pdf_text) > 17 {
		save_extracted_err := write_string_to_file(q_file_extracted, pdf_text)
		if save_extracted_err != nil {
			log_error.Tracef("save_extracted_err = %v", save_extracted_err)
		}
	}

	rd := ResultData{
		Identifier:        identifier,
		URL:               source_url,
		DataDir:           recordDir,
		TotalPages:        int64(info.Pages),
		URLChecksum:       pdf_url_checksum,
		PDFChecksum:       checksum,
		PDFPath:           q_file_pdf,
		OCRTextPath:       q_file_ocr,
		ExtractedTextPath: q_file_extracted,
		RecordPath:        q_file_record,
		Info:              *info,
		Metadata:          metadata,
	}
	err = WriteResultDataToJson(rd)
	if err != nil {
		return err
	}
	sm_resultdatas.Store(identifier, rd)
	sm_documents.Store(identifier, Document{
		Identifier:          identifier,
		URL:                 source_url,
		Pages:               make(map[int64]Page),
		TotalPages:          int64(info.Pages),
		CoverPageIdentifier: "",
		Collection:          Collection{},
	})
	a_i_total_documents.Add(1)
	log_info.Printf("sending URL %v (rd struct) into the ch_ImportedRow channel", rd.URL)
	err = ch_ImportedRow.Write(rd)
	if err != nil {
		log_error.Tracef("cant write to ch_ImportedRow: %+v", err)
		return err
	}
	return nil
}

func process_import_pdf(ctx context.Context, path string, metadata_json string) error {
	//log.Printf("using ctx %v to process_import_pdf", ctx.Value(CtxKey("filename")))
	basename := filepath.Base(path)
	pdf_url_checksum := Sha256(basename)
	identifier := NewIdentifier(6)

	recordDir := filepath.Join(*flag_s_database_directory, pdf_url_checksum)
	err := os.MkdirAll(recordDir, 0750)
	if err != nil {
		log_error.Fatalf("cannot mkdir -p %v due to err %v", recordDir, err)
		return err
	}

	var (
		q_file_pdf       = filepath.Join(recordDir, strings.ReplaceAll(basename, `/`, `_`))
		q_file_ocr       = filepath.Join(recordDir, "ocr.txt")
		q_file_extracted = filepath.Join(recordDir, "extracted.txt")
		q_file_record    = filepath.Join(recordDir, "record.json")
	)

	original, original_open_err := os.Open(path)
	if original_open_err != nil {
		return original_open_err
	}

	original_stat, original_stat_err := os.Stat(path)
	if original_stat_err != nil {
		return original_stat_err
	}

	destination, destination_err := os.Create(q_file_pdf)
	if destination_err != nil {
		return destination_err
	}

	var bufferSize int64 = 8192 // 8MB
	buffer := make([]byte, bufferSize)

	if original_stat.Size() > bufferSize {
		// more than 8MB in size, use the buffer approach
		for {
			bytes_read, read_err := original.Read(buffer)
			if read_err != nil {
				if read_err == io.EOF {
					break // End of file reached, exit the loop
				}
				return read_err
			}
			_, write_err := destination.Write(buffer[:bytes_read])
			if write_err != nil {
				return write_err
			}
		}
	} else {
		_, copy_err := io.Copy(destination, original)
		if copy_err != nil {
			return copy_err
		}
	}

	close_original_err := original.Close()
	if close_original_err != nil {
		return close_original_err
	}
	close_destination_err := destination.Close()
	if close_destination_err != nil {
		return close_destination_err
	}

	//log.Println("process_import_pdf() q_pdf_file = " + q_file_pdf)

	// [-TO-DO-]: first the downloaded file must be scanned through a virus scanner, this will introduce a runtime requirement release process update
	// TODO: ensure clamav is installed via the release upgrade script
	if !*flag_b_disable_clamav {
		output, action_taken, clamav_scan_err := scan_path_with_clam_av(q_file_pdf)
		if clamav_scan_err != nil {
			log_debug.Tracef("while scanning %v clamav scan returned an err: %v", q_file_pdf, clamav_scan_err)
			return clamav_scan_err
		}

		if action_taken {
			log_debug.Tracef("action taken against %v with clamav: %v", q_file_pdf, output)
			return fmt.Errorf("antivirus action taken against %v", q_file_pdf)
		}
	}

	// [-TO-DO-]: analyze the metadata of the pdf file to determine totalPages, currently defaulting to 0
	pdf_analysis, pdf_analysis_err := analyze_pdf_path(q_file_pdf)
	if pdf_analysis_err != nil {
		log_debug.Tracef("received an err %v on pdf_analysis for %v \n\n %+v", pdf_analysis_err, q_file_pdf, pdf_analysis)
		return pdf_analysis_err
	}

	var info *PDFCPUInfoResponseInfo
	if len(pdf_analysis.Infos) > 0 {
		data := pdf_analysis.Infos[0] // capture
		info = &data                  // point
	}

	if info == nil {
		info = &PDFCPUInfoResponseInfo{Pages: 0}
	}

	var embedded_text string
	if info != nil {
		if info.Pages > 0 {
			a_i_total_pages.Add(int64(info.Pages))
		}
		if len(info.Keywords) > 0 {
			embedded_text = strings.Join(info.Keywords, " ")
			embedded_text = strings.ReplaceAll(embedded_text, "  ", " ")
		}
	}

	pdfFile, pdfFileErr := os.Open(q_file_pdf) // [-TO-DO-]: need to add some security around this process
	if pdfFileErr != nil {
		return pdfFileErr
	}
	checksum := FileSha512(pdfFile)
	pdf_file_close_err := pdfFile.Close()
	if pdf_file_close_err != nil {
		return pdf_file_close_err
	}

	metadata := make(map[string]string)
	if len(metadata_json) > 0 {
		metadata_bytes := bytes.NewBufferString(metadata_json).Bytes()
		err = json.Unmarshal(metadata_bytes, &metadata)
		metadata_bytes = nil
		if err != nil {
			log_debug.Tracef("failed to parse the --metadata-json due to err %v", err)
		}
	}

	pdf_text, pdf_text_err := extract_text_from_pdf(q_file_pdf)
	if pdf_text_err != nil {
		log_error.Tracef("pdf_text_err = %v", pdf_text_err)
	}

	//log.Printf("comparing pdf_text to embedded_text")

	if len(embedded_text) > 17 {
		save_extracted_err := write_string_to_file(q_file_extracted, embedded_text)
		if save_extracted_err != nil {
			log_error.Tracef("save_extracted_err = %v", save_extracted_err)
		}
		info.Keywords = []string{} // flush memory
	} else if len(pdf_text) > 17 {
		save_extracted_err := write_string_to_file(q_file_extracted, pdf_text)
		if save_extracted_err != nil {
			log_error.Tracef("save_extracted_err = %v", save_extracted_err)
		}
	}

	rd := ResultData{
		Identifier:        identifier,
		DataDir:           recordDir,
		TotalPages:        int64(info.Pages),
		URLChecksum:       pdf_url_checksum,
		PDFChecksum:       checksum,
		PDFPath:           q_file_pdf,
		OCRTextPath:       q_file_ocr,
		ExtractedTextPath: q_file_extracted,
		RecordPath:        q_file_record,
		Info:              *info,
		Metadata:          metadata,
	}
	err = WriteResultDataToJson(rd)
	if err != nil {
		return err
	}
	sm_resultdatas.Store(identifier, rd)
	sm_documents.Store(identifier, Document{
		Identifier:          identifier,
		Pages:               make(map[int64]Page),
		TotalPages:          int64(info.Pages),
		CoverPageIdentifier: "",
		Collection:          Collection{},
	})
	a_i_total_documents.Add(1)
	log_info.Printf("sending URL %v (rd struct) into the ch_ImportedRow channel", rd.URL)
	err = ch_ImportedRow.Write(rd)
	if err != nil {
		log_error.Tracef("cant write to ch_ImportedRow: %+v", err)
		return err
	}
	return nil
}

func process_import_directory(ctx context.Context, directory string) error {
	wg := countable_waitgroup.CountableWaitGroup{}
	err := filepath.Walk(directory, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(info.Name(), ".pdf") {
			wg.Add(1)
			go func(wg *countable_waitgroup.CountableWaitGroup) {
				defer wg.Done()
				process_err := process_import_pdf(ctx, path, "")
				if process_err != nil {
					return
				}
			}(&wg)
		}

		return nil
	})
	if err != nil {
		return err
	}
	wg.Wait()
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

func repair_pdf(path string) error {
	dir_path := filepath.Dir(path)
	filename := filepath.Base(path)
	format := `gs -sDEVICE=pdfwrite -dCompatibilityLevel=1.5 -dPDFSETTINGS=/prepress -dNOPAUSE -dQUIET -dBATCH -sOutputFile=%s %s`
	source_path := strings.Clone(path)
	dest_path := filepath.Join(dir_path, "repaired_"+filename)
	command := fmt.Sprintf(format, dest_path, source_path)
	cmd := exec.Command(command)
	var out bytes.Buffer
	var err bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &err
	runErr := cmd.Run()
	if runErr != nil || strings.Contains(string(err.Bytes()), `err`) {
		return runErr
	}
	source_rm_err := os.Remove(source_path)
	if source_rm_err != nil {
		return source_rm_err
	}
	rename_err := os.Rename(dest_path, source_path)
	if rename_err != nil {
		return rename_err
	}
	return nil
}

func repair_then_analyze_pdf(path string) (PDFCPUInfoResponse, error) {
	repair_err := repair_pdf(path)
	if repair_err != nil {
		return PDFCPUInfoResponse{}, repair_err
	}
	return analyze_pdf_path(path)

}

// analyze_pdf_path uses the `pdfcpu` utility to determine properties about a PDF file.
func analyze_pdf_path(path string) (PDFCPUInfoResponse, error) {
	cmd := exec.Command("pdfcpu", "info", "-json", path)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return PDFCPUInfoResponse{}, err
	}
	outStr := string(out.Bytes())
	jsonStartIndex := strings.Index(outStr, "{")
	if jsonStartIndex == -1 {
		return PDFCPUInfoResponse{}, fmt.Errorf("no JSON content found in pdfcpu output")
	}

	cleanJSON := outStr[jsonStartIndex:]
	outStr = strings.TrimSpace(cleanJSON)
	var pdf_info PDFCPUInfoResponse
	pdf_info_err := json.Unmarshal([]byte(outStr), &pdf_info)
	if pdf_info_err != nil {
		return PDFCPUInfoResponse{}, pdf_info_err
	}
	return pdf_info, nil
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
	var rowData []Column
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
				log_debug.Tracef("skipping rowField %v due to headerFields not matching up properly", rowFields[i])
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
			a_i_total_documents.Add(1)
			callbackErr := callback(ctx, populatedRow)
			if callbackErr != nil {
				log_debug.Tracef("failed to insert row %v with error %v", populatedRow, callbackErr)
			}
		}
	}
}
