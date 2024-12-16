package main

import (
	"path/filepath"
	"runtime"

	"github.com/andreimerlescu/configurable"
)

var (
	config = configurable.New()

	// Use a -config <file> to define every property here as needed in one place
	flag_s_config_file = config.NewString("config", "", "Path to the configuration file")

	// OR you can use these properties as `/apario/bin/writer -<prop> "<val>"` when running the binary
	flag_g_log_file           = config.NewString("log", filepath.Join(".", "logs", "writer.log"), "File to save logs to. Default is logs/engine-YYYY-MM-DD-HH-MM-SS.log")
	flag_s_database_directory = config.NewString("database-directory", "", "the database directory for the apario-reader instance to consume")

	// Performance Tuning
	flag_i_sem_limiter = config.NewInt("limit", channel_buffer_size, "Number of rows to concurrently process.")
	flag_i_buffer      = config.NewInt("buffer", reader_buffer_bytes, "Memory allocation for CSV buffer (min 168 * 1024 = 168KB)")

	// Antivirus
	flag_b_disable_clamav = config.NewBool("no-clam", false, "disable clamav antivirus scanning of downloaded files")

	// Single PDF ingestion options
	flag_s_pdf_title        = config.NewString("pdf-title", "", "title of the document")
	flag_s_metadata_columns = config.NewString("csv-metadata-columns", "", "comma separated value of header values that represent metadata ; saved as key => value where key is the column header")
	flag_s_download_pdf_url = config.NewString("download-pdf-url", "", "url of pdf to download. must start with http or https and must be an application/pdf type less than 369MB in size")
	flag_s_import_pdf_path  = config.NewString("import-pdf-path", "", "relative path to the pdf that will be processed that are less than 369MB")
	flag_s_import_directory = config.NewString("import-directory", "", "absolute path to a directory that will import all .pdf files that are less than 369MB")

	// Import .xlsx collections
	flag_s_import_xlsx               = config.NewString("import-xlsx", "", "relative path to an excel spreadsheet where sheet 1 is a table of urls and metadata properties. use additional args to associate columns to key data points.")
	flag_s_xlsx_path_directory       = config.NewString("xlsx-path-directory", "", "absolute path to the directory containing the filenames in the Path column of the XLSX file")
	flag_s_xlsx_column_url           = config.NewString("xlsx-column-url", "", "value of row 1 whose column correlates to URLs to download PDF files from")
	flag_s_xlsx_column_path          = config.NewString("xlsx-column-path", "", "value of row 1 whose column correlates to absolute paths of PDF files")
	flag_s_xlsx_column_record_number = config.NewString("xlsx-column-record-number", "", "value of row 1 whose column correlates to a unique record identifier or number")
	flag_s_xlsx_column_title         = config.NewString("xlsx-column-title", "", "value of row 1 whose column correlates to the title of the document")

	// Import .csv collections
	flag_s_import_csv               = config.NewString("import-csv", "", "relative path to an excel spreadsheet where output is a comma separated table of urls and metadata properties. use additional args to associate columns to key data points.")
	flag_s_csv_column_url           = config.NewString("csv-column-url", "", "value of row 1 whose column correlates to URLs to download PDF files from")
	flag_s_csv_column_path          = config.NewString("csv-column-path", "", "value of row 1 whose column correlates to absolute paths of PDF files")
	flag_s_csv_column_record_number = config.NewString("csv-column-record-number", "", "value of row 1 whose column correlates to a unique record identifier or number")
	flag_s_csv_column_title         = config.NewString("csv-column-title", "", "value of row 1 whose column correlates to the title of the document")
	flag_s_pdf_metadata_json        = config.NewString("metadata-json", "", "json key value map[string]string")

	// Runtime appliance control levers
	flag_g_jpg_quality      = config.NewInt("jpeg-quality", 96, "Quality percentage (as int 1-100) for compressing PNG images into JPEG files.")
	flag_g_progressive_jpeg = config.NewBool("progressive", true, "Convert compressed JPEG images into progressive images.")

	// Network Intensive Tasks (higher values could result in throttling or IP banning - recommended value: 1)
	flag_b_sem_download = config.NewInt("download", 1, "Semaphore Limiter for downloading PDF files from URLs.")

	// IO Intensive Tasks - High Intensity
	flag_b_sem_tesseract = config.NewInt("tesseract", 1, "Semaphore Limiter for `tesseract` binary.")                     // tesseract uses all threads available
	flag_b_sem_pdftotext = config.NewInt("pdftotext", runtime.GOMAXPROCS(0), "Semaphore Limiter for `pdftotext` binary.") // single threaded
	flag_b_sem_pdftoppm  = config.NewInt("pdftoppm", runtime.GOMAXPROCS(0), "Semaphore Limiter for `pdftoppm` binary.")   // single threaded
	// IO Intensive Tasks - Medium Intensity
	flag_g_sem_png2jpg   = config.NewInt("png2jpg", 33, "Semaphore Limiter for converting PNG images to JPG.")
	flag_g_sem_wjsonfile = config.NewInt("wjsonfile", 33, "Semaphore Limiter for writing a JSON file to disk.")
	// IO Intensive Tasks - Low Intensity
	flag_g_sem_filedata = config.NewInt("filedata", 333, "Semaphore Limiter for writing metadata about a processed file to JSON.")
	flag_g_sem_shafile  = config.NewInt("shafile", 333, "Semaphore Limiter for calculating the SHA256 checksum of files.")

	// Compute Intensive Tasks - High Intensity
	flag_b_sem_pdfcpu  = config.NewInt("pdfcpu", 3, "Semaphore Limiter for `pdfcpu` binary.")
	flag_b_sem_gs      = config.NewInt("gs", 3, "Semaphore Limiter for `gs` binary.")
	flag_b_sem_convert = config.NewInt("convert", 3, "Semaphore Limiter for `convert` binary.")
	// Compute Intensive Tasks - Medium Intensity
	flag_g_sem_resize    = config.NewInt("resize", 66, "Semaphore Limiter for resize PNG or JPG images.")
	flag_g_sem_darkimage = config.NewInt("darkimage", 66, "Semaphore Limiter for converting an image to dark mode.")
	// Compute Intensive Tasks - Low Intensity
	flag_g_sem_watermark = config.NewInt("watermark", 999, "Semaphore Limiter for adding a watermark to an image.")
	flag_g_sem_shastring = config.NewInt("shastring", 999, "Semaphore Limiter for calculating the SHA256 checksum of a string.")
)
