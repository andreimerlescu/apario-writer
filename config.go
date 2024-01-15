package main

import (
	`fmt`
	`path/filepath`

	con `github.com/andreimerlescu/configurable`
)

var (
	config = con.New()

	// Command Line Flags
	flag_s_config_file               = config.NewString("config", "", "Path to the configuration file")
	flag_s_download_pdf_url          = config.NewString("download-pdf-url", "", "url of pdf to download. must start with http or https and must be an application/pdf type less than 369MB in size")
	flag_s_import_pdf_path           = config.NewString("import-pdf-path", "", "relative path to the pdf that will be processed that are less than 369MB")
	flag_s_import_directory          = config.NewString("import-directory", "", "absolute path to a directory that will import all .pdf files that are less than 369MB")
	flag_s_import_xlsx               = config.NewString("import-xlsx", "", "relative path to an excel spreadsheet where sheet 1 is a table of urls and metadata properties. use additional args to associate columns to key data points.")
	flag_s_xlsx_column_url           = config.NewString("xlsx-column-url", "", "value of row 1 whose column correlates to URLs to download PDF files from")
	flag_s_xlsx_column_path          = config.NewString("xlsx-column-path", "", "value of row 1 whose column correlates to absolute paths of PDF files")
	flag_s_xlsx_column_record_number = config.NewString("xlsx-column-record-number", "", "value of row 1 whose column correlates to a unique record identifier or number")
	flag_s_xlsx_column_title         = config.NewString("xlsx-column-title", "", "value of row 1 whose column correlates to the title of the document")
	flag_s_import_csv                = config.NewString("import-csv", "", "relative path to an excel spreadsheet where output is a comma separated table of urls and metadata properties. use additional args to associate columns to key data points.")
	flag_s_csv_column_url            = config.NewString("csv-column-url", "", "value of row 1 whose column correlates to URLs to download PDF files from")
	flag_s_csv_column_path           = config.NewString("csv-column-path", "", "value of row 1 whose column correlates to absolute paths of PDF files")
	flag_s_csv_column_record_number  = config.NewString("csv-column-record-number", "", "value of row 1 whose column correlates to a unique record identifier or number")
	flag_s_csv_column_title          = config.NewString("csv-column-title", "", "value of row 1 whose column correlates to the title of the document")
	flag_s_metadata_columns          = config.NewString("csv-metadata-columns", "", "comma separated value of header values that represent metadata ; saved as key => value where key is the column header")
	flag_b_disable_clamav            = config.NewBool("no-clam", false, "disable clamav antivirus scanning of downloaded files")
	flag_s_pdf_title                 = config.NewString("pdf-title", "", "title of the document")
	flag_s_pdf_metadata_json         = config.NewString("metadata-json", "", "json key value map[string]string")
	flag_s_database_directory        = config.NewString("database-directory", "", "the database directory for the apario-reader instance to consume")
	flag_i_sem_limiter               = config.NewInt("limit", channel_buffer_size, "Number of rows to concurrently process.")
	flag_i_buffer                    = config.NewInt("buffer", reader_buffer_bytes, "Memory allocation for CSV buffer (min 168 * 1024 = 168KB)")
	flag_b_sem_tesseract             = config.NewInt("tesseract", 1, "Semaphore Limiter for `tesseract` binary.")
	flag_b_sem_download              = config.NewInt("download", 2, "Semaphore Limiter for downloading PDF files from URLs.")
	flag_b_sem_pdfcpu                = config.NewInt("pdfcpu", 17, "Semaphore Limiter for `pdfcpu` binary.")
	flag_b_sem_gs                    = config.NewInt("gs", 17, "Semaphore Limiter for `gs` binary.")
	flag_b_sem_pdftotext             = config.NewInt("pdftotext", 17, "Semaphore Limiter for `pdftotext` binary.")
	flag_b_sem_convert               = config.NewInt("convert", 17, "Semaphore Limiter for `convert` binary.")
	flag_b_sem_pdftoppm              = config.NewInt("pdftoppm", 17, "Semaphore Limiter for `pdftoppm` binary.")
	flag_g_sem_png2jpg               = config.NewInt("png2jpg", 17, "Semaphore Limiter for converting PNG images to JPG.")
	flag_g_sem_resize                = config.NewInt("resize", 17, "Semaphore Limiter for resize PNG or JPG images.")
	flag_g_sem_shafile               = config.NewInt("shafile", 36, "Semaphore Limiter for calculating the SHA256 checksum of files.")
	flag_g_sem_watermark             = config.NewInt("watermark", 36, "Semaphore Limiter for adding a watermark to an image.")
	flag_g_sem_darkimage             = config.NewInt("darkimage", 36, "Semaphore Limiter for converting an image to dark mode.")
	flag_g_sem_filedata              = config.NewInt("filedata", 369, "Semaphore Limiter for writing metadata about a processed file to JSON.")
	flag_g_sem_shastring             = config.NewInt("shastring", 369, "Semaphore Limiter for calculating the SHA256 checksum of a string.")
	flag_g_sem_wjsonfile             = config.NewInt("wjsonfile", 369, "Semaphore Limiter for writing a JSON file to disk.")
	flag_g_jpg_quality               = config.NewInt("jpeg-quality", 71, "Quality percentage (as int 1-100) for compressing PNG images into JPEG files.")
	flag_g_progressive_jpeg          = config.NewBool("progressive", true, "Convert compressed JPEG images into progressive images.")
	flag_g_log_file                  = config.NewString("log", filepath.Join(".", "logs", fmt.Sprintf("apario-writer-%04d-%02d-%02d-%02d-%02d-%02d.log", startedAt.Year(), startedAt.Month(), startedAt.Day(), startedAt.Hour(), startedAt.Minute(), startedAt.Second())), "File to save logs to. Default is logs/engine-YYYY-MM-DD-HH-MM-SS.log")
)
