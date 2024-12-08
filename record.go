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
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const jfk_pdf_download_prefix = "https://www.archives.gov/files/research/jfk/releases/"

func process_custom_csv_row(ctx context.Context, row []Column) error {
	var source_url string
	var local_path string
	metadata_columns := strings.Split(strings.ReplaceAll(*flag_s_metadata_columns, ` `, ``), ",")
	metadata := make(map[string]string)
	for _, column := range row {
		if column.Header == *flag_s_csv_column_path {
			local_path = column.Value
			break
		} else if column.Header == *flag_s_csv_column_url {
			source_url = column.Value
			break
		} else {
			for _, meta_column := range metadata_columns {
				if strings.ToLower(column.Header) == strings.ToLower(meta_column) {
					metadata[column.Header] = column.Value
					break
				}
			}
			continue
		}
	}
	metadata_marshal, marshal_err := json.Marshal(metadata)
	if marshal_err != nil {
		return marshal_err
	}
	if len(source_url) == 0 && len(local_path) > 0 {
		// use local_path
		err := process_import_pdf(ctx, local_path, string(metadata_marshal))
		if err != nil {
			return err
		}
		return nil
	} else if len(source_url) > 0 && len(local_path) == 0 {
		// use source_url
		err := process_download_pdf(ctx, source_url, string(metadata_marshal))
		if err != nil {
			return err
		}
		return nil
	} else if len(source_url) == 0 && len(local_path) == 0 {
		// both empty
		return fmt.Errorf("failed to find columns for source_url or local_path in row")
	}
	return fmt.Errorf("failed to process_custom_csv_row due to bad data; skipping row")
}

func processRecord(ctx context.Context, row []Column) error {
	log.Printf("processRecord received for row %v", row)

	loadedFile := fmt.Sprintf("%s", ctx.Value(CtxKey("filename")))

	var totalPages int64 = 0
	var filename, title, collection, pdf_url, source_url, comments, record_number, to_name, from_name, agency string
	var creation_date, release_date time.Time

	// header fields for different files
	// stargate = checksum|filename|type|bytes|title|collection|document_number|release_decision|original_classification|page_count|creation_date|release_date|sequence_number|bogus_date|case_number|pdf_url|source_url
	// jfk2023b = File Name,Record Num,NARA Release Date,Formerly Withheld,Agency,Doc Date,Doc Type,File Num,To Name,From Name,Title,Num Pages,Originator,Record Series,Review Date,Comments,Pages Released
	// jfk2022 = File Name,Record Num,NARA Release Date,Formerly Withheld,Doc Date,Doc Type,File Num,To Name,From Name,Title,Num Pages,Originator,Record Series,Review Date,Comments,Pages Released
	// jfk2021 = Record Number,File Title,NARA Release Date,Formerly Withheld,Document Date,Document Type,File Number,To,From,Title,Original Document Pages,Originator,Record Series,Review Date,Comments,Document Pages in PDF
	// jfk2018 = File Name,Record Num,NARA Release Date,Formerly Withheld,Agency,Doc Date,Doc Type,File Num,To Name,From Name,Title,Num Pages,Originator,Record Series,Review Date,Comments,Pages Released

	// if the column Path does not contain a / indicating its an absolute path, then use the PathDirectory property to get the path of the local PDF to the writer
	var pdf_local_path string
	if !strings.HasPrefix(*flag_s_xlsx_column_path, `/`) {
		pdf_local_path = strings.Clone(*flag_s_xlsx_path_directory + *flag_s_xlsx_column_path)
	}

	var dateErr error
	for _, r := range row {
		switch r.Header {
		case "filename", "File Name", *flag_s_xlsx_column_path:
			filename = r.Value
		case "title", "Title", "File Title", *flag_s_xlsx_column_title:
			title = r.Value
		case "Comments":
			comments = r.Value
		case "To Name", "To":
			to_name = r.Value
		case "From Name", "From":
			from_name = r.Value
		case "collection", "Record Series":
			collection = r.Value
		case "pdf_url":
			pdf_url = r.Value
		case "document_number", "Record Num", *flag_s_xlsx_column_record_number:
			record_number = r.Value
		case "Agency":
			agency = r.Value
		case "source_url", *flag_s_xlsx_column_url:
			source_url = r.Value
		case "creation_date", "Doc Date", "Document Date":
			creation_date, dateErr = parseDateString(r.Value)
			if dateErr != nil {
				log.Printf("failed to parse the release date %v due to error %v", r.Value, dateErr)
			}
		case "release_date", "NARA Release Date":
			release_date, dateErr = parseDateString(r.Value)
			if dateErr != nil {
				log.Printf("failed to parse the release date %v due to error %v", r.Value, dateErr)
			}
		case "page_count", "Num Pages", "Original Document Pages":
			pg, err := strconv.Atoi(r.Value)
			if err == nil {
				totalPages += int64(pg)
			}
		}
	}

	// when no TotalPages column is available, determine the total using pdfinfo binary
	if totalPages == 0 && len(pdf_local_path) > 0 {
		cmd := exec.CommandContext(ctx, "sh", "-s", fmt.Sprintf("pdfinfo %s | grep Pages | awk '{print $2}'", pdf_local_path))
		var out bytes.Buffer
		cmd.Stdout = &out
		err := cmd.Run()
		if err != nil {
			log.Printf("Error running command: %v\n", err)
			return err
		}
		tp := strings.TrimSpace(out.String())
		tpi, ie := strconv.Atoi(tp)
		if ie != nil {
			log.Printf("Error converting pages to int: %v", ie)
			return ie
		}
		totalPages = int64(tpi)
	}

	a_i_total_pages.Add(totalPages)

	if !strings.HasPrefix(pdf_url, "http") && strings.Contains(loadedFile, "jfk") {
		pdf_url = jfk_pdf_download_prefix + filename
		log.Printf("pdf_url = %v", pdf_url)
	}

	if !strings.HasPrefix(source_url, "http") {
		if len(source_url) == 0 {
			// not set
			source_url = pdf_url
		} else {
			// has a value, but it doesnt begin with http
			log.Printf("ERROR: source_url doesn't begin with http but has a value of %v", source_url)
		}
	}

	pdf_url_checksum := Sha256(pdf_url)

	identifier := NewIdentifier(6)

	recordDir := filepath.Join(*flag_s_database_directory, pdf_url_checksum)
	err := os.MkdirAll(recordDir, 0750)
	if err != nil {
		return err
	}

	var (
		q_file_pdf       = filepath.Join(recordDir, strings.ReplaceAll(filename, string(os.PathSeparator), `_`))
		q_file_ocr       = filepath.Join(recordDir, "ocr.txt")
		q_file_extracted = filepath.Join(recordDir, "extracted.txt")
		q_file_record    = filepath.Join(recordDir, "record.json")
	)

	_, downloadedPdfErr := os.Stat(q_file_pdf)
	if os.IsNotExist(downloadedPdfErr) {
		log.Printf("downloading URL %v to %v", pdf_url, q_file_pdf)
		err = downloadFile(ctx, pdf_url, q_file_pdf)
		if err != nil {
			return err
		}
	}

	pdfFile, pdfFileErr := os.Open(q_file_pdf)
	if pdfFileErr != nil {
		return pdfFileErr
	}
	checksum := FileSha512(pdfFile)
	pdfFile.Close()

	metadata := make(map[string]string)
	if len(title) > 0 {
		metadata["title"] = title
	}
	if len(comments) > 0 {
		metadata["comments"] = comments
	}
	if creation_date != (time.Time{}) {
		metadata["created_at"] = creation_date.Format("2006-01-02")
	}
	if release_date != (time.Time{}) {
		metadata["released_at"] = release_date.Format("2006-01-02")
	}
	if len(to_name) > 0 {
		metadata["to_name"] = to_name
	}
	if len(from_name) > 0 {
		metadata["from_name"] = from_name
	}
	if len(agency) > 0 {
		metadata["agency"] = agency
	}
	if len(record_number) > 0 {
		metadata["record_number"] = record_number
	}
	if len(collection) > 0 {
		metadata["collection"] = collection
	}
	rd := ResultData{
		Identifier:        identifier,
		URL:               pdf_url,
		DataDir:           recordDir,
		TotalPages:        totalPages,
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
	sm_resultdatas.Store(identifier, rd)
	sm_documents.Store(identifier, Document{
		Identifier:          identifier,
		URL:                 source_url,
		Pages:               make(map[int64]Page),
		TotalPages:          int64(totalPages),
		CoverPageIdentifier: "",
		Collection:          Collection{},
		mu:                  &sync.Mutex{},
	})
	log.Printf("sending URL %v (rd struct) into the ch_ImportedRow channel", rd.URL)
	err = ch_ImportedRow.Write(rd)
	if err != nil {
		log.Printf("cant write to ch_ImportedRow")
		return err
	}
	return nil
}
